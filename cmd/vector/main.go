package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"net"

	pb "github.com/chongs12/enterprise-knowledge-base/api/proto/vector"
	"github.com/chongs12/enterprise-knowledge-base/internal/common/models"
	"github.com/chongs12/enterprise-knowledge-base/internal/embedding"
	"github.com/chongs12/enterprise-knowledge-base/internal/vector"
	"github.com/chongs12/enterprise-knowledge-base/pkg/config"
	"github.com/chongs12/enterprise-knowledge-base/pkg/database"
	"github.com/chongs12/enterprise-knowledge-base/pkg/logger"
	"github.com/chongs12/enterprise-knowledge-base/pkg/metrics"
	"github.com/chongs12/enterprise-knowledge-base/pkg/middleware"
	"github.com/chongs12/enterprise-knowledge-base/pkg/rabbitmq"
	"github.com/chongs12/enterprise-knowledge-base/pkg/tracing"
	milvus "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	// 文件功能：向量服务入口，初始化配置、数据库、Ark 嵌入器与 Milvus 存储，启动 HTTP 路由
	// 作者：system
	// 创建日期：2025-11-26；修改日期：2025-11-26
	ctx := context.Background()

	// 加载配置：包含 Ark/Milvus/Redis/JWT/Server 等
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志：JSON 格式，便于集中采集与检索
	logger.Init()
	logger.Info(ctx, "Starting vector service", "service", "vector", "environment", cfg.Server.Mode)

	// 初始化数据库连接：Gorm + MySQL
	db, err := database.Init(&cfg.Database)
	if err != nil {
		logger.Error(ctx, "Failed to initialize database", "error", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	// 自动迁移：分块表结构（TextChunk）
	if err = db.AutoMigrate(&models.TextChunk{}); err != nil {
		logger.Error(ctx, "Failed to migrate database", "error", err.Error())
		os.Exit(1)
	}

	// Initialize Tracing
	jaegerEndpoint := os.Getenv("JAEGER_ENDPOINT")
	if jaegerEndpoint == "" {
		jaegerEndpoint = "localhost:4317"
	}
	shutdown, err := tracing.InitTracer("vector-service", jaegerEndpoint)
	if err != nil {
		logger.Error(ctx, "Failed to init tracer", "error", err.Error())
		os.Exit(1)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			logger.Error(ctx, "Failed to shutdown tracer", "error", err.Error())
		}
	}()

	// 优先使用 Ark + Milvus 方案
	// 初始化 Ark 嵌入器：用于生成文本向量
	emb, err := embedding.NewArkEmbedder(cfg.Ark.APIKey, cfg.Ark.Model, cfg.Ark.BaseURL, cfg.Ark.Region)
	if err != nil {
		logger.Error(ctx, "Failed to initialize Ark embedder", "error", err.Error())
		os.Exit(1)
	}
	// 初始化 Milvus 客户端：向量存储与检索
	mcli, err := milvus.NewClient(ctx, milvus.Config{Address: cfg.Milvus.Addr, Username: cfg.Milvus.Username, Password: cfg.Milvus.Password})
	if err != nil {
		logger.Error(ctx, "Failed to initialize Milvus client", "error", err.Error())
		os.Exit(1)
	}
	// 创建 Milvus 存储（索引与检索）
	mstore, err := vector.NewMilvusStore(ctx, mcli, cfg.Milvus.Collection, cfg.Ark.APIKey, cfg.Ark.Model, cfg.Ark.BaseURL, cfg.Ark.Region, cfg.Milvus.VectorField, cfg.Milvus.VectorDim, cfg.Milvus.VectorType)
	if err != nil {
		logger.Error(ctx, "Failed to initialize Milvus store", "error", err.Error())
		os.Exit(1)
	}
	// 诊断日志：打印当前配置与集合字段信息，便于校验向量字段/维度
	logger.Info(ctx, "Vector config", "field", cfg.Milvus.VectorField, "dim", cfg.Milvus.VectorDim, "type", cfg.Milvus.VectorType)
	_ = mstore.LogDiagnostics(ctx)
	// 探测 Ark 向量维度（仅日志）：用于发现模型输出维度与集合不一致的问题
	if emb != nil {
		if v, e := emb.Embed(ctx, []string{"diagnose"}); e == nil && len(v) > 0 {
			logger.Info(ctx, "Ark embedding probe", "dim", len(v[0]))
			// 维度校验：与 Milvus 集合配置不一致时记录错误并退出，避免后续入库失败
			if len(v[0]) != cfg.Milvus.VectorDim {
				logger.Error(ctx, "Embedding dim mismatch", "ark_dim", len(v[0]), "milvus_dim", cfg.Milvus.VectorDim)
				os.Exit(1)
			}
		} else if e != nil {
			logger.Warn(ctx, "Ark embedding probe failed", "error", e.Error())
		}
	}
	rdb := redis.NewClient(&redis.Options{Addr: fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port), Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	vectorService := vector.NewVectorService(db, emb, mstore, rdb)
	vectorHandler := vector.NewHandler(vectorService)

	// Start RabbitMQ Consumer
	go func() {
		mqClient, err := rabbitmq.NewClient(cfg.RabbitMQ.URL, cfg.RabbitMQ.Queue)
		if err != nil {
			logger.Error(ctx, "Failed to connect to RabbitMQ", "error", err.Error())
			return
		}
		defer mqClient.Close()

		msgs, err := mqClient.Consume()
		if err != nil {
			logger.Error(ctx, "Failed to start consumer", "error", err.Error())
			return
		}

		logger.Info(ctx, "Started RabbitMQ consumer")

		for d := range msgs {
			var payload struct {
				DocumentID string `json:"document_id"`
				Content    string `json:"content"`
				ChunkSize  int    `json:"chunk_size"`
			}
			if err := json.Unmarshal(d.Body, &payload); err != nil {
				logger.Error(ctx, "Failed to unmarshal message", "error", err.Error())
				d.Nack(false, false)
				continue
			}

			logger.Info(ctx, "Processing document from MQ", "document_id", payload.DocumentID)
			pCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			if err := vectorService.ProcessDocument(pCtx, payload.DocumentID, payload.Content, payload.ChunkSize); err != nil {
				logger.Error(ctx, "Failed to process document", "error", err.Error(), "document_id", payload.DocumentID)
				cancel()
				d.Nack(false, false) // Requeue or dead-letter
			} else {
				logger.Info(ctx, "Successfully processed document", "document_id", payload.DocumentID)
				d.Ack(false)
				cancel()
			}
		}
	}()

	// Start gRPC server
	go func() {
		grpcPort := os.Getenv("EKB_VECTOR_GRPC_PORT")
		if grpcPort == "" {
			grpcPort = "50053"
		}
		lis, err := net.Listen("tcp", ":"+grpcPort)
		if err != nil {
			logger.Fatalf("failed to listen: %v", err)
		}
		s := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
		pb.RegisterVectorServiceServer(s, vector.NewGRPCServer(vectorService))
		logger.Info(ctx, "Starting gRPC server", "port", grpcPort)
		if err := s.Serve(lis); err != nil {
			logger.Fatalf("failed to serve: %v", err)
		}
	}()

	// 初始化鉴权中间件：保护私有接口
	authMiddleware := middleware.NewAuthMiddleware(cfg.JWT.Secret)

	// 配置 Gin 路由：日志与恢复中间件
	if cfg.Server.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(otelgin.Middleware("vector-service"))
	hm := metrics.NewHTTPMetrics(metrics.DefaultRegistry(), "ekb", "vector")
	router.Use(metrics.MetricsMiddleware("vector", hm))
	router.GET("/metrics", gin.WrapH(metrics.MetricsHandler(metrics.DefaultRegistry())))

	// 健康检查端点：便于探针与监控
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "vector",
			"timestamp": time.Now().Unix(),
		})
	})

	// 注册业务路由
	vectorHandler.SetupRoutes(router, authMiddleware)

	// 创建并启动 HTTP 服务器
	// 支持独立端口环境变量覆盖（EKB_VECTOR_PORT）；为空时回退到通用 server.port
	port := os.Getenv("EKB_VECTOR_PORT")
	if port == "" {
		port = cfg.Server.Port
	}
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router,
	}

	// 异步启动服务器，主线程监听退出信号
	go func() {
		logger.Info(ctx, "Starting HTTP server", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Failed to start server", "error", err.Error())
			os.Exit(1)
		}
	}()

	// 等待中断信号，执行优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "Shutting down server...")

	// 为未完成请求提供 30 秒的关闭窗口
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx, "Server forced to shutdown", "error", err.Error())
	}

	logger.Info(ctx, "Server exited")
}
