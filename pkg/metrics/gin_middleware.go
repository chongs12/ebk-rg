package metrics

import (
    "time"

    "github.com/gin-gonic/gin"
)

func MetricsMiddleware(service string, hm *HTTPMetrics) gin.HandlerFunc {
    return func(c *gin.Context) {
        hm.InflightRequests.WithLabelValues(service).Inc()
        start := time.Now()
        c.Next()
        dur := time.Since(start).Seconds()
        route := c.FullPath()
        if route == "" {
            route = "unknown"
        }
        status := c.Writer.Status()
        st := "";
        switch {
        case status >= 500:
            st = "5xx"
        case status >= 400:
            st = "4xx"
        default:
            st = "2xx"
        }
        hm.RequestsTotal.WithLabelValues(service, route, c.Request.Method, st).Inc()
        hm.RequestDuration.WithLabelValues(service, route, c.Request.Method, st).Observe(dur)
        hm.InflightRequests.WithLabelValues(service).Dec()
    }
}
