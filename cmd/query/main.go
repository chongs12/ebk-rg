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
    "github.com/chongs12/enterprise-knowledge-base/internal/embedding"
    "github.com/chongs12/enterprise-knowledge-base/internal/rag_query"
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

    if err = db.AutoMigrate(&models.TextChunk{}, &models.Query{}); err != nil {
        logger.Error(ctx, "Failed to migrate database", "error", err.Error())
        os.Exit(1)
    }

    // 初始化 Ark 嵌入器与 Milvus 存储，构建向量服务供检索
    emb, err := embedding.NewArkEmbedder(cfg.Ark.APIKey, cfg.Ark.Model, cfg.Ark.BaseURL, cfg.Ark.Region)
    if err != nil {
        logger.Error(ctx, "Failed to initialize Ark embedder", "error", err.Error())
        os.Exit(1)
    }
    mcli, err := milvus.NewClient(ctx, milvus.Config{Address: cfg.Milvus.Addr, Username: cfg.Milvus.Username, Password: cfg.Milvus.Password})
    if err != nil {
        logger.Error(ctx, "Failed to initialize Milvus client", "error", err.Error())
        os.Exit(1)
    }
    mstore, err := vector.NewMilvusStore(ctx, mcli, cfg.Milvus.Collection, cfg.Ark.APIKey, cfg.Ark.Model, cfg.Ark.BaseURL, cfg.Ark.Region, cfg.Milvus.VectorField, cfg.Milvus.VectorDim, cfg.Milvus.VectorType)
    if err != nil {
        logger.Error(ctx, "Failed to initialize Milvus store", "error", err.Error())
        os.Exit(1)
    }
    _ = mstore.LogDiagnostics(ctx)
    rdb := redis.NewClient(&redis.Options{Addr: fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port), Password: cfg.Redis.Password, DB: cfg.Redis.DB})
    vs := vector.NewVectorService(db, emb, mstore, rdb)

    // 初始化 Ark ChatModel 作为 LLM
    chat, err := arkmodel.NewChatModel(ctx, &arkmodel.ChatModelConfig{
        APIKey:   cfg.Ark.APIKey,
        Model:    cfg.RagQuery.Model,
        BaseURL:  cfg.Ark.BaseURL,
        Region:   cfg.Ark.Region,
        MaxTokens: &[]int{cfg.RagQuery.Parameters.MaxTokens}[0],
        Temperature: &[]float32{float32(cfg.RagQuery.Parameters.Temperature)}[0],
    })
    if err != nil {
        logger.Error(ctx, "Failed to initialize Ark ChatModel", "error", err.Error())
        os.Exit(1)
    }

    // 构建 RAG 服务与路由
    ragService := rag_query.NewRAGQueryService(db, rdb, vs, chat)
    ragHandler := rag_query.NewHandler(ragService)

    authMiddleware := middleware.NewAuthMiddleware(cfg.JWT.Secret)
    if cfg.Server.Mode == "production" { gin.SetMode(gin.ReleaseMode) }
    router := gin.New()
    router.Use(gin.Logger())
    router.Use(gin.Recovery())

    router.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "rag_query", "timestamp": time.Now().Unix()})
    })

    ragHandler.SetupRoutes(router, authMiddleware)

    srv := &http.Server{Addr: fmt.Sprintf(":%s", cfg.Server.Port), Handler: router}
    go func() {
        logger.Info(ctx, "Starting HTTP server", "port", cfg.Server.Port)
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

