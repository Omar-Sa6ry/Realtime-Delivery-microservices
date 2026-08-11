package metrics

import (
	"fmt"
	"net/http"
	"time"
)

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriterWrapper(w http.ResponseWriter) *responseWriterWrapper {
	return &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}
}

func (w *responseWriterWrapper) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// HTTPMetricsMiddleware wraps an HTTP handler to capture request counts, durations, and status codes for Prometheus
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do not track metrics scraping endpoint
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		startTime := time.Now()

		wrapper := newResponseWriterWrapper(w)
		next.ServeHTTP(wrapper, r)

		duration := time.Since(startTime).Seconds()
		statusCodeStr := fmt.Sprintf("%d", wrapper.statusCode)

		// Record request count and duration
		RequestCounter.WithLabelValues("HTTP", r.Method, r.URL.Path, statusCodeStr).Inc()
		RequestDuration.WithLabelValues("HTTP", r.Method, r.URL.Path, statusCodeStr).Observe(duration)

		// Record error metric if status code is an error (>= 400)
		if wrapper.statusCode >= 400 {
			ErrorCounter.WithLabelValues("HTTP:"+r.URL.Path, statusCodeStr).Inc()
		}
	})
}
