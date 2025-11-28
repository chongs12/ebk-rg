package middleware

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

func TestRequestIDGenerated(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(RequestID())
    r.GET("/t", func(c *gin.Context) { c.Status(http.StatusOK) })

    req := httptest.NewRequest(http.MethodGet, "/t", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    rid := w.Header().Get("X-Request-ID")
    if rid == "" {
        t.Fatalf("missing X-Request-ID")
    }
    if _, err := uuid.Parse(rid); err != nil {
        t.Fatalf("invalid X-Request-ID: %v", err)
    }
}

func TestRequestIDPropagate(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(RequestID())
    r.GET("/t", func(c *gin.Context) { c.Status(http.StatusOK) })

    req := httptest.NewRequest(http.MethodGet, "/t", nil)
    req.Header.Set("X-Request-ID", "11111111-1111-1111-1111-111111111111")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    rid := w.Header().Get("X-Request-ID")
    if rid != "11111111-1111-1111-1111-111111111111" {
        t.Fatalf("request id not propagated: %s", rid)
    }
}
