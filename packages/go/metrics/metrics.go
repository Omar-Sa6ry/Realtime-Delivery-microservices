package metrics

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// RequestCounter measures the total requests processed
	RequestCounter *prometheus.CounterVec

	// RequestDuration measures request processing duration in seconds
	RequestDuration *prometheus.HistogramVec

	// ErrorCounter measures total application errors
	ErrorCounter *prometheus.CounterVec

	registerOnce sync.Once
)

func init() {
	RegisterMetrics()
}

// RegisterMetrics registers application Prometheus metrics with the default registry
func RegisterMetrics() {
	registerOnce.Do(func() {
		RequestCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "app_requests_total",
				Help: "Total number of requests processed (HTTP, GraphQL, gRPC)",
			},
			[]string{"protocol", "method", "path", "statusCode"},
		)

		RequestDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "app_request_duration_seconds",
				Help:    "Duration of requests in seconds",
				Buckets: []float64{0.01, 0.05, 0.1, 0.2, 0.5, 1, 2, 5},
			},
			[]string{"protocol", "method", "path", "statusCode"},
		)

		ErrorCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "app_errors_total",
				Help: "Total number of application errors",
			},
			[]string{"context", "errorCode"},
		)

		prometheus.MustRegister(RequestCounter)
		prometheus.MustRegister(RequestDuration)
		prometheus.MustRegister(ErrorCounter)
	})
}

// HTTPHandler returns the default prometheus metrics HTTP handler
func HTTPHandler() http.Handler {
	return promhttp.Handler()
}

// StartMetricsServer starts a standalone HTTP server to expose Prometheus metrics
func StartMetricsServer(port string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", HTTPHandler())
	
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: mux,
	}

	return server.ListenAndServe()
}
