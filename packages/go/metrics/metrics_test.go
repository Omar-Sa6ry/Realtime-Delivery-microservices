package metrics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestRegisterMetricsIsIdempotent(t *testing.T) {
	RegisterMetrics()

	require.NotNil(t, RequestCounter)
	require.NotNil(t, RequestDuration)
	require.NotNil(t, ErrorCounter)

	// A second call must not panic on duplicate registration.
	RegisterMetrics()
}

func TestHTTPHandlerExposesMetrics(t *testing.T) {
	HTTPMetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/scrape-probe", nil))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	HTTPHandler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "app_requests_total")
	assert.Contains(t, body, "app_request_duration_seconds")
}

func TestHTTPHandlerBodyIsReadable(t *testing.T) {
	rec := httptest.NewRecorder()
	HTTPHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, body)
}

func TestHTTPMetricsMiddlewareSkipsScrapeEndpoint(t *testing.T) {
	labels := []string{"HTTP", http.MethodGet, "/metrics", "200"}
	before := testutil.ToFloat64(RequestCounter.WithLabelValues(labels...))

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	HTTPMetricsMiddleware(next).ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/metrics", nil))

	after := testutil.ToFloat64(RequestCounter.WithLabelValues(labels...))
	assert.Equal(t, before, after, "/metrics must not be recorded")
}

func TestHTTPMetricsMiddlewareRecordsSuccess(t *testing.T) {
	labels := []string{"HTTP", http.MethodPost, "/v1/deliveries", "201"}
	before := testutil.ToFloat64(RequestCounter.WithLabelValues(labels...))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/deliveries", strings.NewReader(""))

	HTTPMetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, before+1, testutil.ToFloat64(RequestCounter.WithLabelValues(labels...)))
}

func TestHTTPMetricsMiddlewareRecordsErrors(t *testing.T) {
	labels := []string{"HTTP:/v1/missing", "404"}
	before := testutil.ToFloat64(ErrorCounter.WithLabelValues(labels...))

	HTTPMetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/missing", nil))

	assert.Equal(t, before+1, testutil.ToFloat64(ErrorCounter.WithLabelValues(labels...)))
}

func TestHTTPMetricsMiddlewareDefaultsToOK(t *testing.T) {
	labels := []string{"HTTP", http.MethodGet, "/v1/plain", "200"}
	before := testutil.ToFloat64(RequestCounter.WithLabelValues(labels...))

	HTTPMetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/plain", nil))

	assert.Equal(t, before+1, testutil.ToFloat64(RequestCounter.WithLabelValues(labels...)))
}

func TestUnaryServerMetricsInterceptorSuccess(t *testing.T) {
	errLabels := []string{"gRPC:/svc/Get", "OK"}
	reqLabels := []string{"gRPC", "RPC", "/svc/Get", "OK"}
	beforeErr := testutil.ToFloat64(ErrorCounter.WithLabelValues(errLabels...))
	beforeReq := testutil.ToFloat64(RequestCounter.WithLabelValues(reqLabels...))

	resp, err := UnaryServerMetricsInterceptor()(context.Background(), "req",
		&grpc.UnaryServerInfo{FullMethod: "/svc/Get"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return "resp", nil
		})

	require.NoError(t, err)
	assert.Equal(t, "resp", resp)
	assert.Equal(t, beforeErr, testutil.ToFloat64(ErrorCounter.WithLabelValues(errLabels...)))
	assert.Equal(t, beforeReq+1, testutil.ToFloat64(RequestCounter.WithLabelValues(reqLabels...)))
}

func TestUnaryServerMetricsInterceptorRecordsError(t *testing.T) {
	boom := errors.New("boom")

	errLabels := []string{"gRPC:/svc/Fail", "UNKNOWN"}
	reqLabels := []string{"gRPC", "RPC", "/svc/Fail", "UNKNOWN"}
	beforeErr := testutil.ToFloat64(ErrorCounter.WithLabelValues(errLabels...))
	beforeReq := testutil.ToFloat64(RequestCounter.WithLabelValues(reqLabels...))

	_, err := UnaryServerMetricsInterceptor()(context.Background(), "req",
		&grpc.UnaryServerInfo{FullMethod: "/svc/Fail"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, boom
		})

	require.ErrorIs(t, err, boom)
	assert.Equal(t, beforeErr+1, testutil.ToFloat64(ErrorCounter.WithLabelValues(errLabels...)))
	assert.Equal(t, beforeReq+1, testutil.ToFloat64(RequestCounter.WithLabelValues(reqLabels...)))
}

func TestStreamServerMetricsInterceptor(t *testing.T) {
	info := &grpc.StreamServerInfo{FullMethod: "/svc/Stream"}

	t.Run("success", func(t *testing.T) {
		reqLabels := []string{"gRPC", "STREAM", "/svc/Stream", "OK"}
		errLabels := []string{"gRPC:/svc/Stream", "OK"}
		beforeReq := testutil.ToFloat64(RequestCounter.WithLabelValues(reqLabels...))
		beforeErr := testutil.ToFloat64(ErrorCounter.WithLabelValues(errLabels...))

		err := StreamServerMetricsInterceptor()(nil, nil, info,
			func(srv interface{}, stream grpc.ServerStream) error { return nil })

		require.NoError(t, err)
		assert.Equal(t, beforeReq+1, testutil.ToFloat64(RequestCounter.WithLabelValues(reqLabels...)))
		assert.Equal(t, beforeErr, testutil.ToFloat64(ErrorCounter.WithLabelValues(errLabels...)))
	})

	t.Run("error", func(t *testing.T) {
		boom := errors.New("stream failed")
		reqLabels := []string{"gRPC", "STREAM", "/svc/Stream", "UNKNOWN"}
		errLabels := []string{"gRPC:/svc/Stream", "UNKNOWN"}
		beforeReq := testutil.ToFloat64(RequestCounter.WithLabelValues(reqLabels...))
		beforeErr := testutil.ToFloat64(ErrorCounter.WithLabelValues(errLabels...))

		err := StreamServerMetricsInterceptor()(nil, nil, info,
			func(srv interface{}, stream grpc.ServerStream) error { return boom })

		require.ErrorIs(t, err, boom)
		assert.Equal(t, beforeReq+1, testutil.ToFloat64(RequestCounter.WithLabelValues(reqLabels...)))
		assert.Equal(t, beforeErr+1, testutil.ToFloat64(ErrorCounter.WithLabelValues(errLabels...)))
	})
}

func TestStartMetricsServerRejectsInvalidPort(t *testing.T) {
	err := StartMetricsServer("not-a-port")
	require.Error(t, err)
	assert.NotEmpty(t, fmt.Sprint(err))
}
