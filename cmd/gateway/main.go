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
    "github.com/chongs12/enterprise-knowledge-base/pkg/metrics"
)

func main() {
	ctx := context.Background()
	// 文件功能：网关服务入口，统一代理路由并提供鉴权、限流与重试
	// 作者：system
	// 创建日期：2025-11-26；修改日期：2025-11-26
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
    router.Use(middleware.RequestID())
    hm := metrics.NewHTTPMetrics(metrics.DefaultRegistry(), "ekb", "gateway")
    router.Use(metrics.MetricsMiddleware("gateway", hm))
    router.GET("/metrics", gin.WrapH(metrics.MetricsHandler(metrics.DefaultRegistry())))

	// 简易限流中间件（令牌桶）：针对每个客户端 IP 限制每秒请求数
	// 变量说明：
	// - buckets：维护每个 IP 的桶状态，容量与填充速率固定
	// - capacity：桶容量
	// - refill：每次补充的令牌数量/周期
	capacity := 50
	buckets := map[string]int{}
	router.Use(func(c *gin.Context) {
		ip := c.ClientIP()
		if _, ok := buckets[ip]; !ok {
			buckets[ip] = capacity
		}
		if buckets[ip] <= 0 {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}
		buckets[ip]--
		// 简易补充：每次请求少量回填，实际可使用定时器或 Redis
		if buckets[ip] < capacity {
			buckets[ip]++
		}
		c.Next()
	})

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
	protected.Any("/documents", docProxy)
	protected.Any("/documents/*proxyPath", docProxy)
	protected.Any("/vectors/*proxyPath", vecProxy)
	protected.Any("/rag/*proxyPath", ragProxy)

	// 支持独立端口环境变量覆盖（EKB_GATEWAY_PORT）；为空时回退到通用 server.port
	port := os.Getenv("EKB_GATEWAY_PORT")
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

// makeProxy 生成上游代理处理器
// 参数：
// - base：上游服务基础地址
// - stripPrefix：保留字段（当前未使用，可扩展路径重写）
// 逻辑：
// - 构造上游请求，透传 headers 与 body
// - 使用简易重试（指数退避）增强鲁棒性
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
		// 简易重试：最多 3 次，指数退避
		var resp *http.Response
		for attempt := 0; attempt < 3; attempt++ {
			resp, err = client.Do(req)
			if err == nil {
				break
			}
			time.Sleep(time.Duration(1<<attempt) * 200 * time.Millisecond)
		}
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
