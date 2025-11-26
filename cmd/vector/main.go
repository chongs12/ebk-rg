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

	"github.com/chongs12/enterprise-knowledge-base/internal/common/models"
	"github.com/chongs12/enterprise-knowledge-base/internal/embedding"
	"github.com/chongs12/enterprise-knowledge-base/internal/vector"
	"github.com/chongs12/enterprise-knowledge-base/pkg/config"
	"github.com/chongs12/enterprise-knowledge-base/pkg/database"
	"github.com/chongs12/enterprise-knowledge-base/pkg/logger"
	"github.com/chongs12/enterprise-knowledge-base/pkg/middleware"
	milvus "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger.Init()
	logger.Info(ctx, "Starting vector service", "service", "vector", "environment", cfg.Server.Mode)

	// Initialize database
	db, err := database.Init(&cfg.Database)
	if err != nil {
		logger.Error(ctx, "Failed to initialize database", "error", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	// Auto migrate database tables
	if err = db.AutoMigrate(&models.TextChunk{}); err != nil {
		logger.Error(ctx, "Failed to migrate database", "error", err.Error())
		os.Exit(1)
	}

	// Prefer Ark + Milvus
	// 初始化 Ark 嵌入器
	emb, err := embedding.NewArkEmbedder(cfg.Ark.APIKey, cfg.Ark.Model, cfg.Ark.BaseURL, cfg.Ark.Region)
	if err != nil {
		logger.Error(ctx, "Failed to initialize Ark embedder", "error", err.Error())
		os.Exit(1)
	}
	// 初始化 Milvus 客户端
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
	// 诊断日志：打印当前配置与集合字段信息
	logger.Info(ctx, "Vector config", "field", cfg.Milvus.VectorField, "dim", cfg.Milvus.VectorDim, "type", cfg.Milvus.VectorType)
	_ = mstore.LogDiagnostics(ctx)
	// 探测 Ark 向量维度（仅日志）
	if emb != nil {
		if v, e := emb.Embed(ctx, []string{"diagnose"}); e == nil && len(v) > 0 {
			logger.Info(ctx, "Ark embedding probe", "dim", len(v[0]))
		} else if e != nil {
			logger.Warn(ctx, "Ark embedding probe failed", "error", e.Error())
		}
	}
	rdb := redis.NewClient(&redis.Options{Addr: fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port), Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	vectorService := vector.NewVectorService(db, emb, mstore, rdb)
	vectorHandler := vector.NewHandler(vectorService)

	// Initialize auth middleware
	authMiddleware := middleware.NewAuthMiddleware(cfg.JWT.Secret)

	// Setup Gin router
	if cfg.Server.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "vector",
			"timestamp": time.Now().Unix(),
		})
	})

	// Setup routes
	vectorHandler.SetupRoutes(router, authMiddleware)

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Server.Port),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		logger.Info(ctx, "Starting HTTP server", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Failed to start server", "error", err.Error())
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "Shutting down server...")

	// Give outstanding requests a 30-second deadline to complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx, "Server forced to shutdown", "error", err.Error())
	}

	logger.Info(ctx, "Server exited")
}
