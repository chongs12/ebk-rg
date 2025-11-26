package main

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
    "os/signal"
    "strings"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"

    "github.com/chongs12/enterprise-knowledge-base/pkg/config"
    "github.com/chongs12/enterprise-knowledge-base/pkg/logger"
    "github.com/chongs12/enterprise-knowledge-base/pkg/middleware"
)

func main() {
    ctx := context.Background()
    cfg, err := config.Load()
    if err != nil {
        fmt.Printf("Failed to load configuration: %v\n", err)
        os.Exit(1)
    }

    logger.Init()
    logger.Info(ctx, "Starting gateway service", "service", "gateway", "environment", cfg.Server.Mode)

    if cfg.Server.Mode == "production" {
        gin.SetMode(gin.ReleaseMode)
    }
    router := gin.New()
    router.Use(gin.Logger())
    router.Use(gin.Recovery())

    router.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "gateway", "timestamp": time.Now().Unix()})
    })

    authProxy := makeProxy(cfg.Gateway.AuthBaseURL, "/api/v1/auth")
    docProxy := makeProxy(cfg.Gateway.DocumentBaseURL, "/api/v1/documents")
    vecProxy := makeProxy(cfg.Gateway.VectorBaseURL, "/api/v1/vectors")
    ragProxy := makeProxy(cfg.Gateway.QueryBaseURL, "/api/v1/rag")

    router.Any("/api/v1/auth/*proxyPath", authProxy)

    authMiddleware := middleware.NewAuthMiddleware(cfg.JWT.Secret)
    protected := router.Group("/api/v1")
    protected.Use(authMiddleware.RequireAuth())
    protected.Any("/documents/*proxyPath", docProxy)
    protected.Any("/vectors/*proxyPath", vecProxy)
    protected.Any("/rag/*proxyPath", ragProxy)

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

func makeProxy(base string, stripPrefix string) gin.HandlerFunc {
    target, _ := url.Parse(base)
    client := &http.Client{Timeout: 60 * time.Second}
    return func(c *gin.Context) {
        u := *target
        u.Path = strings.TrimSuffix(u.Path, "/") + c.Request.URL.Path
        u.RawQuery = c.Request.URL.RawQuery

        req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, u.String(), c.Request.Body)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
            return
        }
        for k, vv := range c.Request.Header {
            for _, v := range vv {
                req.Header.Add(k, v)
            }
        }
        resp, err := client.Do(req)
        if err != nil {
            c.JSON(http.StatusBadGateway, gin.H{"error": "upstream unavailable"})
            return
        }
        defer resp.Body.Close()
        for k, vv := range resp.Header {
            for _, v := range vv {
                c.Writer.Header().Add(k, v)
            }
        }
        c.Status(resp.StatusCode)
        _, _ = io.Copy(c.Writer, resp.Body)
    }
}

