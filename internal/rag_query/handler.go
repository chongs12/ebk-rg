package rag_query

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"

    "github.com/chongs12/enterprise-knowledge-base/pkg/logger"
    "github.com/chongs12/enterprise-knowledge-base/pkg/middleware"
    "github.com/chongs12/enterprise-knowledge-base/pkg/metrics"
)

// Handler RAG查询模块的路由处理器
type Handler struct {
    service *RAGQueryService
}

var ragBM = metrics.NewBusinessMetrics(metrics.DefaultRegistry(), "ekb")

// NewHandler 创建处理器
func NewHandler(service *RAGQueryService) *Handler { return &Handler{service: service} }

// RAGRequest 请求体
type RAGRequest struct {
	Query       string  `json:"query" binding:"required"`
	Limit       int     `json:"limit"`
	Temperature float32 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	SessionID   string  `json:"session_id"`
}

// Ask 同步查询生成
func (h *Handler) Ask(c *gin.Context) {
    ctx := c.Request.Context()

	uidRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	var uid uuid.UUID
	switch v := uidRaw.(type) {
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
			return
		}
		uid = id
	case uuid.UUID:
		uid = v
	default:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id type"})
		return
	}

	var req RAGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 {
		req.Limit = 5
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 1024
	}
	if req.Temperature <= 0 {
		req.Temperature = 0.7
	}

    start := time.Now()
    res, err := h.service.AskSync(ctx, uid, &RAGQueryRequest{Query: req.Query, Limit: req.Limit, Temperature: req.Temperature, MaxTokens: req.MaxTokens, SessionID: req.SessionID, AuthToken: c.GetHeader("Authorization")})
    if err != nil {
        logger.Error(ctx, "RAG Ask failed", "error", err.Error())
        ragBM.RagQueryTotal.WithLabelValues("query", "fail").Inc()
        ragBM.RagQueryDuration.WithLabelValues("query", "fail").Observe(time.Since(start).Seconds())
        c.JSON(http.StatusInternalServerError, gin.H{"error": "rag ask failed"})
        return
    }
    c.JSON(http.StatusOK, gin.H{
        "answer":     res.Answer,
        "sources":    res.Sources,
        "usage":      res.Usage,
        "latency_ms": time.Since(start).Milliseconds(),
    })
    ragBM.RagQueryTotal.WithLabelValues("query", "success").Inc()
    ragBM.RagQueryDuration.WithLabelValues("query", "success").Observe(time.Since(start).Seconds())
}

// AskStream 异步流式查询生成（SSE）
func (h *Handler) AskStream(c *gin.Context) {
	ctx := c.Request.Context()

	uidRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	var uid uuid.UUID
	switch v := uidRaw.(type) {
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
			return
		}
		uid = id
	case uuid.UUID:
		uid = v
	default:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id type"})
		return
	}

	var req RAGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 {
		req.Limit = 5
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 1024
	}
	if req.Temperature <= 0 {
		req.Temperature = 0.7
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	stream, errs := h.service.AskStream(ctx, uid, &RAGQueryRequest{Query: req.Query, Limit: req.Limit, Temperature: req.Temperature, MaxTokens: req.MaxTokens, SessionID: req.SessionID, AuthToken: c.GetHeader("Authorization")})
	c.Status(http.StatusOK)

	// 心跳：每 5 秒发送 ping 事件，提示客户端维持连接
	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case chunk, ok := <-stream:
			if !ok {
				return
			}
			_, _ = c.Writer.Write([]byte("data: " + chunk + "\n\n"))
			c.Writer.Flush()
		case err, ok := <-errs:
			if !ok {
				return
			}
			logger.Error(ctx, "RAG stream error", "error", err.Error())
			_, _ = c.Writer.Write([]byte("event: error\n" + "data: " + err.Error() + "\n\n"))
			c.Writer.Flush()
			return
		case <-heartbeat.C:
			_, _ = c.Writer.Write([]byte("event: ping\n" + "data: heartbeat\n\n"))
			c.Writer.Flush()
		case <-ctx.Done():
			return
		}
	}
}

// SetupRoutes 注册路由
func (h *Handler) SetupRoutes(router *gin.Engine, authMiddleware *middleware.AuthMiddleware) {
	group := router.Group("/api/v1/rag")
	group.Use(authMiddleware.RequireAuth())
	group.POST("/query", h.Ask)
	group.POST("/query/stream", h.AskStream)
}
