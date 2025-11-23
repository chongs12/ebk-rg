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
	"github.com/chongs12/enterprise-knowledge-base/internal/document"
	"github.com/chongs12/enterprise-knowledge-base/pkg/config"
	"github.com/chongs12/enterprise-knowledge-base/pkg/database"
	"github.com/chongs12/enterprise-knowledge-base/pkg/logger"
	"github.com/chongs12/enterprise-knowledge-base/pkg/middleware"
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
	logger.Info(ctx, "Starting document service", "service", "document", "environment", cfg.Server.Mode)

	// Initialize database
	db, err := database.Init(&cfg.Database)
	if err != nil {
		logger.Error(ctx, "Failed to initialize database", "error", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	// Auto migrate database tables
	if err := db.AutoMigrate(&models.Document{}, &models.DocumentPermission{}); err != nil {
		logger.Error(ctx, "Failed to migrate database", "error", err.Error())
		os.Exit(1)
	}

	// Initialize document service
	docService := document.NewDocumentService(db, cfg.Storage.UploadPath, cfg.Storage.MaxFileSize, cfg.Storage.AllowedTypes)
	docHandler := document.NewHandler(docService)

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
			"service":   "document",
			"timestamp": time.Now().Unix(),
		})
	})

	// Setup routes
	docHandler.SetupRoutes(router, authMiddleware)

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error(ctx, "Server forced to shutdown", "error", err.Error())
	}

	logger.Info(ctx, "Server exited")
}