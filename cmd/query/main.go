package main

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"

    arkmodel "github.com/cloudwego/eino-ext/components/model/ark"

    "github.com/chongs12/enterprise-knowledge-base/internal/common/models"
    "github.com/chongs12/enterprise-knowledge-base/internal/rag_query"
    "github.com/chongs12/enterprise-knowledge-base/pkg/config"
    "github.com/chongs12/enterprise-knowledge-base/pkg/database"
    "github.com/chongs12/enterprise-knowledge-base/pkg/logger"
    "github.com/chongs12/enterprise-knowledge-base/pkg/middleware"
    "github.com/chongs12/enterprise-knowledge-base/pkg/metrics"
    "github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()
	// 文件功能：RAG 查询服务入口，初始化配置、数据库、Redis 与 Ark ChatModel，注册路由
	// 作者：system
	// 创建日期：2025-11-26；修改日期：2025-11-26

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	logger.Init()
	logger.Info(ctx, "Starting query service", "service", "rag_query", "environment", cfg.Server.Mode)

	db, err := database.Init(&cfg.Database)
	if err != nil {
		logger.Error(ctx, "Failed to initialize database", "error", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	if err = db.AutoMigrate(&models.Query{}); err != nil {
		logger.Error(ctx, "Failed to migrate database", "error", err.Error())
		os.Exit(1)
	}

	rdb := redis.NewClient(&redis.Options{Addr: fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port), Password: cfg.Redis.Password, DB: cfg.Redis.DB})

	// 初始化 Ark ChatModel 作为 LLM
	chat, err := arkmodel.NewChatModel(ctx, &arkmodel.ChatModelConfig{
		APIKey:      cfg.Ark.APIKey,
		Model:       cfg.RagQuery.Model,
		BaseURL:     cfg.Ark.BaseURL,
		Region:      cfg.Ark.Region,
		MaxTokens:   &[]int{cfg.RagQuery.Parameters.MaxTokens}[0],
		Temperature: &[]float32{float32(cfg.RagQuery.Parameters.Temperature)}[0],
	})
	if err != nil {
		logger.Error(ctx, "Failed to initialize Ark ChatModel", "error", err.Error())
		os.Exit(1)
	}

	// 构建 RAG 服务与路由
	httpc := &http.Client{Timeout: 30 * time.Second}
	ragService := rag_query.NewRAGQueryService(db, rdb, httpc, cfg.Gateway.EntryBaseURL, chat)
	ragHandler := rag_query.NewHandler(ragService)

	authMiddleware := middleware.NewAuthMiddleware(cfg.JWT.Secret)
	if cfg.Server.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
    router := gin.New()
    router.Use(gin.Logger())
    router.Use(gin.Recovery())
    router.Use(middleware.RequestID())
    hm := metrics.NewHTTPMetrics(metrics.DefaultRegistry(), "ekb", "query")
    router.Use(metrics.MetricsMiddleware("query", hm))
    router.GET("/metrics", gin.WrapH(metrics.MetricsHandler(metrics.DefaultRegistry())))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "rag_query", "timestamp": time.Now().Unix()})
	})

	ragHandler.SetupRoutes(router, authMiddleware)

	// 支持独立端口环境变量覆盖（EKB_QUERY_PORT）；为空时回退到通用 server.port
	port := os.Getenv("EKB_QUERY_PORT")
	if port == "" {
		port = cfg.Server.Port
	}
	srv := &http.Server{Addr: fmt.Sprintf(":%s", port), Handler: router}
	go func() {
		logger.Info(ctx, "Starting HTTP server", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Failed to start server", "error", err.Error())
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info(ctx, "Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx, "Server forced to shutdown", "error", err.Error())
	}
	logger.Info(ctx, "Server exited")
}
