package metrics

import (
    "sync"

    "github.com/prometheus/client_golang/prometheus"
)

var (
    defaultRegistry     *prometheus.Registry
    onceDefaultRegistry sync.Once
)

func DefaultRegistry() *prometheus.Registry {
    onceDefaultRegistry.Do(func() {
        r := prometheus.NewRegistry()
        r.MustRegister(prometheus.NewGoCollector())
        r.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
        defaultRegistry = r
    })
    return defaultRegistry
}

type HTTPMetrics struct {
    RequestsTotal        *prometheus.CounterVec
    RequestDuration      *prometheus.HistogramVec
    InflightRequests     *prometheus.GaugeVec
}

func NewHTTPMetrics(reg *prometheus.Registry, namespace, service string) *HTTPMetrics {
    reqTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
        Namespace: namespace,
        Name:      "http_requests_total",
        Help:      "Total number of HTTP requests",
    }, []string{"service", "route", "method", "status"})
    reqDur := prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Namespace: namespace,
        Name:      "http_request_duration_seconds",
        Help:      "HTTP request duration in seconds",
        Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
    }, []string{"service", "route", "method", "status"})
    inflight := prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: namespace,
        Name:      "http_inflight_requests",
        Help:      "Current number of inflight HTTP requests",
    }, []string{"service"})

    reg.MustRegister(reqTotal, reqDur, inflight)
    inflight.WithLabelValues(service).Set(0)

    return &HTTPMetrics{
        RequestsTotal:    reqTotal,
        RequestDuration:  reqDur,
        InflightRequests: inflight,
    }
}

type BusinessMetrics struct {
    UploadTotal       *prometheus.CounterVec
    UploadDuration    *prometheus.HistogramVec
    VectorizeTotal    *prometheus.CounterVec
    VectorizeDuration *prometheus.HistogramVec
    RagQueryTotal     *prometheus.CounterVec
    RagQueryDuration  *prometheus.HistogramVec
}

func NewBusinessMetrics(reg *prometheus.Registry, namespace string) *BusinessMetrics {
    mkCounter := func(name, help string) *prometheus.CounterVec {
        c := prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Name: name, Help: help}, []string{"service", "status"})
        reg.MustRegister(c)
        return c
    }
    mkHist := func(name, help string) *prometheus.HistogramVec {
        h := prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: namespace, Name: name, Help: help, Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}}, []string{"service", "status"})
        reg.MustRegister(h)
        return h
    }
    return &BusinessMetrics{
        UploadTotal:       mkCounter("upload_total", "Total uploads"),
        UploadDuration:    mkHist("upload_duration_seconds", "Upload duration in seconds"),
        VectorizeTotal:    mkCounter("vectorize_total", "Total vectorize operations"),
        VectorizeDuration: mkHist("vectorize_duration_seconds", "Vectorize duration in seconds"),
        RagQueryTotal:     mkCounter("rag_query_total", "Total RAG queries"),
        RagQueryDuration:  mkHist("rag_query_duration_seconds", "RAG query duration in seconds"),
    }
}
