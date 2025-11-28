package middleware

import (
    "context"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/chongs12/enterprise-knowledge-base/pkg/logger"
    oteltrace "go.opentelemetry.io/otel/trace"
)

func RequestID() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        rid := c.GetHeader("X-Request-ID")
        if rid == "" {
            rid = uuid.New().String()
        }
        c.Set("request_id", rid)
        c.Writer.Header().Set("X-Request-ID", rid)
        c.Request.Header.Set("X-Request-ID", rid)
        c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "request_id", rid))
        sp := oteltrace.SpanFromContext(c.Request.Context())
        sc := sp.SpanContext()
        if sc.TraceID().IsValid() {
            c.Writer.Header().Set("X-Trace-ID", sc.TraceID().String())
        }
        logger.Info(c.Request.Context(), "incoming request", "event_type", "request_in", "method", c.Request.Method, "path", c.Request.URL.Path)
        c.Next()
        c.Writer.Header().Set("X-Request-ID", rid)
        dur := time.Since(start).Milliseconds()
        status := c.Writer.Status()
        st := "success"
        if status >= 400 {
            st = "fail"
        }
        logger.Info(c.Request.Context(), "outgoing response", "event_type", "response_out", "status_code", status, "duration_ms", dur, "status", st)
    }
}

func InjectUserIDToContext(c *gin.Context, userID string) {
    c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "user_id", userID))
}
