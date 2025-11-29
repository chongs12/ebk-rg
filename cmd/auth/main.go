package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chongs12/enterprise-knowledge-base/internal/auth"
	"github.com/chongs12/enterprise-knowledge-base/internal/common/models"
	"github.com/chongs12/enterprise-knowledge-base/pkg/config"
	"github.com/chongs12/enterprise-knowledge-base/pkg/database"
	"github.com/chongs12/enterprise-knowledge-base/pkg/logger"
	"github.com/chongs12/enterprise-knowledge-base/pkg/metrics"
	"github.com/chongs12/enterprise-knowledge-base/pkg/middleware"
	"github.com/gin-gonic/gin"
)

// @title Enterprise Knowledge Base Auth Service API
// @version 1.0
// @description Authentication service for Enterprise Knowledge Base
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8081
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// 文件功能：认证服务入口，初始化配置、数据库与路由；支持端口环境变量覆盖
	// 作者：system
	// 创建日期：2025-11-26；修改日期：2025-11-26
	cfg := config.Get()
	logger.Init()
	ctx := context.Background()

	if cfg.Server.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := database.Init(&cfg.Database)
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := database.AutoMigrate(&models.User{}); err != nil {
		logger.Fatalf("Failed to migrate database: %v", err)
	}

	authService := auth.NewAuthService(
		db,
		cfg.JWT.Secret,
		cfg.JWT.ExpireTime,
		cfg.JWT.ExpireTime*7,
		cfg.JWT.Issuer,
	)

	authMiddleware := middleware.NewAuthMiddleware(cfg.JWT.Secret)
	authHandler := auth.NewAuthHandler(authService, authMiddleware)

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	hm := metrics.NewHTTPMetrics(metrics.DefaultRegistry(), "ekb", "auth")
	router.Use(metrics.MetricsMiddleware("auth", hm))
	router.GET("/metrics", gin.WrapH(metrics.MetricsHandler(metrics.DefaultRegistry())))

	router.GET("/health", authHandler.HealthCheck)

	api := router.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.RefreshToken)
		}

		users := api.Group("/users")
		users.Use(authMiddleware.RequireAuth())
		{
			users.GET("/profile", authHandler.GetProfile)
			users.PUT("/profile", authHandler.UpdateProfile)
			users.PUT("/password", authHandler.ChangePassword)
			users.DELETE("/account", authHandler.DeactivateAccount)
		}
	}

	// 支持独立端口环境变量覆盖（EKB_AUTH_PORT）；为空时回退到通用 server.port
	port := os.Getenv("EKB_AUTH_PORT")
	if port == "" {
		port = cfg.Server.Port
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router,
	}

	go func() {
		logger.Info(ctx, "Starting auth service", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(ctx, "Failed to start server", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "Shutting down auth service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error(ctx, "Server forced to shutdown", "error", err)
	}

	logger.Info(ctx, "Auth service stopped")
}
