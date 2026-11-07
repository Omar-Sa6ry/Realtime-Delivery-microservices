package observability_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/observability"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewMetrics registers everything on prometheus.DefaultRegisterer via
// promauto, so the same instance must be shared by every test in this
// package (a second successful registration would panic).
var (
	metricsOnce   sync.Once
	sharedMetrics *observability.Metrics
)

func metrics(t *testing.T) *observability.Metrics {
	t.Helper()
	metricsOnce.Do(func() {
		sharedMetrics = observability.NewMetrics()
	})
	if sharedMetrics == nil {
		t.Fatal("NewMetrics() returned nil")
	}
	return sharedMetrics
}

// recordAll invokes every Record*/Set* method exactly once.
func recordAll(m *observability.Metrics) {
	m.RecordDispatchRequest("success")
	m.RecordDispatchRequest("failure")
	m.RecordDispatchSuccess("driver-1")
	m.RecordDispatchFailure("no_drivers_available")
	m.RecordDispatchDuration("dispatch", 0.25)
	m.RecordAssignmentCreated("offered")
	m.RecordAssignmentAccepted("driver-1")
	m.RecordAssignmentRejected("driver-1", "busy")
	m.RecordAssignmentExpired("driver-1")
	m.RecordReservationConflict("driver-1")
	m.RecordLocationUpdate("driver-1")
	m.RecordStaleLocation("driver-1")
	m.RecordGeoQueryDuration("geoadd", 0.01)
	m.RecordLockContention("driver-1")
	m.RecordKafkaPublishFailure("drivers.events")
	m.SetKafkaConsumerLag("drivers.events", 65, 42)
	m.RecordNATPublishFailure("driver.location.updated")
	m.RecordGrpcRequestDuration("GetUser", "OK", 0.02)
	m.SetDriverState("driver-1", 2)
	m.SetDriverCount("AVAILABLE", 10)
	m.SetAssignmentState("assignment-1", "driver-1", 3)
}

func TestNewMetricsConstructsAllCollectors(t *testing.T) {
	m := metrics(t)

	collectors := map[string]interface{}{
		"DispatchRequestsTotal":          m.DispatchRequestsTotal,
		"DispatchSuccessTotal":           m.DispatchSuccessTotal,
		"DispatchFailureTotal":           m.DispatchFailureTotal,
		"DispatchDurationSeconds":        m.DispatchDurationSeconds,
		"DriverAssignmentTotal":          m.DriverAssignmentTotal,
		"DriverAssignmentAcceptTotal":    m.DriverAssignmentAcceptTotal,
		"DriverAssignmentRejectTotal":    m.DriverAssignmentRejectTotal,
		"DriverAssignmentExpireTotal":    m.DriverAssignmentExpireTotal,
		"DriverReservationConflictTotal": m.DriverReservationConflictTotal,
		"DriverLocationUpdatesTotal":     m.DriverLocationUpdatesTotal,
		"DriverLocationStaleTotal":       m.DriverLocationStaleTotal,
		"RedisGeoQueryDurationSeconds":   m.RedisGeoQueryDurationSeconds,
		"RedisLockContentionTotal":       m.RedisLockContentionTotal,
		"KafkaPublishFailuresTotal":      m.KafkaPublishFailuresTotal,
		"KafkaConsumerLag":               m.KafkaConsumerLag,
		"NATPublishFailuresTotal":        m.NATPublishFailuresTotal,
		"GrpcRequestDurationSeconds":     m.GrpcRequestDurationSeconds,
		"DriverStateGauge":               m.DriverStateGauge,
		"DriverCountGauge":               m.DriverCountGauge,
		"AssignmentStateGauge":           m.AssignmentStateGauge,
	}
	for name, c := range collectors {
		v := reflect.ValueOf(c)
		if v.Kind() == reflect.Ptr && v.IsNil() {
			t.Errorf("NewMetrics() left %s nil", name)
		}
	}
}

func TestMetricsScrapeExposesAllSeries(t *testing.T) {
	m := metrics(t)
	// recordAll is deliberately invoked only here so that every counter is
	// incremented exactly once for the whole package.
	recordAll(m)

	// Scrape the default registry exactly like Prometheus would.
	handler := promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("scrape status = %d, want 200", rec.Code)
	}
	bodyBytes, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read scrape body: %v", err)
	}
	body := string(bodyBytes)

	// Each metric family must be present.
	metricNames := []string{
		"driver_dispatch_requests_total",
		"driver_dispatch_success_total",
		"driver_dispatch_failure_total",
		"driver_dispatch_duration_seconds",
		"driver_assignment_total",
		"driver_assignment_accept_total",
		"driver_assignment_reject_total",
		"driver_assignment_expire_total",
		"driver_reservation_conflict_total",
		"driver_location_updates_total",
		"driver_location_stale_total",
		"driver_redis_geo_query_duration_seconds",
		"driver_redis_lock_contention_total",
		"driver_kafka_publish_failures_total",
		"driver_kafka_consumer_lag",
		"driver_nats_publish_failures_total",
		"driver_grpc_request_duration_seconds",
		"driver_driver_state",
		"driver_driver_count",
		"driver_assignment_state",
	}
	for _, name := range metricNames {
		if !strings.Contains(body, name) {
			t.Errorf("scrape is missing metric family %q", name)
		}
	}

	// Series with their expected label sets and values.
	series := []struct {
		name string
		want string
	}{
		{`driver_dispatch_requests_total{status="failure"} 1`, ""},
		{`driver_dispatch_requests_total{status="success"} 1`, ""},
		{`driver_dispatch_success_total{driver_id="driver-1"} 1`, ""},
		{`driver_dispatch_failure_total{reason="no_drivers_available"} 1`, ""},
		{`driver_assignment_total{status="offered"} 1`, ""},
		{`driver_assignment_accept_total{driver_id="driver-1"} 1`, ""},
		{`driver_assignment_reject_total{driver_id="driver-1",reason="busy"} 1`, ""},
		{`driver_assignment_expire_total{driver_id="driver-1"} 1`, ""},
		{`driver_reservation_conflict_total{driver_id="driver-1"} 1`, ""},
		{`driver_location_updates_total{driver_id="driver-1"} 1`, ""},
		{`driver_location_stale_total{driver_id="driver-1"} 1`, ""},
		{`driver_redis_lock_contention_total{driver_id="driver-1"} 1`, ""},
		{`driver_kafka_publish_failures_total{topic="drivers.events"} 1`, ""},
		{`driver_nats_publish_failures_total{subject="driver.location.updated"} 1`, ""},
		{`driver_kafka_consumer_lag{partition="A",topic="drivers.events"} 42`, ""},
		{`driver_driver_count{status="AVAILABLE"} 10`, ""},
		{`driver_driver_state{driver_id="driver-1"} 2`, ""},
		{`driver_assignment_state{assignment_id="assignment-1",driver_id="driver-1"} 3`, ""},
	}
	for _, s := range series {
		if !strings.Contains(body, s.name) {
			t.Errorf("scrape is missing series %q", s.name)
		}
	}

	// Histograms: observations must land in the configured buckets.
	histograms := []string{
		`driver_dispatch_duration_seconds_count{operation="dispatch"} 1`,
		`driver_dispatch_duration_seconds_sum{operation="dispatch"} 0.25`,
		`driver_redis_geo_query_duration_seconds_count{operation="geoadd"} 1`,
		`driver_grpc_request_duration_seconds_count{method="GetUser",status="OK"} 1`,
	}
	for _, h := range histograms {
		if !strings.Contains(body, h) {
			t.Errorf("scrape is missing histogram series %q", h)
		}
	}
}

func TestNewMetricsDuplicateRegistrationPanics(t *testing.T) {
	// Ensure the first registration happened, then a second one must panic
	// because promauto registers on the default registerer.
	metrics(t)

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		observability.NewMetrics()
	}()
	if !panicked {
		t.Errorf("second NewMetrics() call did not panic on duplicate registration")
	}
}
