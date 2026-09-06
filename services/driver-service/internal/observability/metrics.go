package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the driver service.
type Metrics struct {
	// Dispatch metrics
	DispatchRequestsTotal     *prometheus.CounterVec
	DispatchSuccessTotal      *prometheus.CounterVec
	DispatchFailureTotal      *prometheus.CounterVec
	DispatchDurationSeconds   *prometheus.HistogramVec

	// Driver assignment metrics
	DriverAssignmentTotal         *prometheus.CounterVec
	DriverAssignmentAcceptTotal   *prometheus.CounterVec
	DriverAssignmentRejectTotal   *prometheus.CounterVec
	DriverAssignmentExpireTotal   *prometheus.CounterVec

	// Driver reservation metrics
	DriverReservationConflictTotal *prometheus.CounterVec

	// Driver location metrics
	DriverLocationUpdatesTotal *prometheus.CounterVec
	DriverLocationStaleTotal   *prometheus.CounterVec

	// Redis metrics
	RedisGeoQueryDurationSeconds *prometheus.HistogramVec
	RedisLockContentionTotal     *prometheus.CounterVec

	// Kafka metrics
	KafkaPublishFailuresTotal *prometheus.CounterVec
	KafkaConsumerLag          *prometheus.GaugeVec

	// NATS metrics
	NATPublishFailuresTotal *prometheus.CounterVec

	// gRPC metrics
	GrpcRequestDurationSeconds *prometheus.HistogramVec

	// Driver state metrics
	DriverStateGauge   *prometheus.GaugeVec
	DriverCountGauge   *prometheus.GaugeVec

	// Assignment state metrics
	AssignmentStateGauge *prometheus.GaugeVec
}

// NewMetrics creates and registers all Prometheus metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		// Dispatch metrics
		DispatchRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "driver_dispatch_requests_total",
				Help: "Total number of dispatch requests",
			},
			[]string{"status"},
		),
		DispatchSuccessTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "driver_dispatch_success_total",
				Help: "Total number of successful dispatches",
			},
			[]string{"driver_id"},
		),
		DispatchFailureTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "driver_dispatch_failure_total",
				Help: "Total number of failed dispatches",
			},
			[]string{"reason"},
		),
		DispatchDurationSeconds: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "driver_dispatch_duration_seconds",
				Help:    "Duration of dispatch operations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),

		// Driver assignment metrics
		DriverAssignmentTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "driver_assignment_total",
				Help: "Total number of driver assignments created",
			},
			[]string{"status"},
		),
		DriverAssignmentAcceptTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "driver_assignment_accept_total",
				Help: "Total number of accepted driver assignments",
			},
			[]string{"driver_id"},
		),
		DriverAssignmentRejectTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "driver_assignment_reject_total",
				Help: "Total number of rejected driver assignments",
			},
			[]string{"driver_id", "reason"},
		),
		DriverAssignmentExpireTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "driver_assignment_expire_total",
				Help: "Total number of expired driver assignments",
			},
			[]string{"driver_id"},
		),

		// Driver reservation metrics
		DriverReservationConflictTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "driver_reservation_conflict_total",
				Help: "Total number of reservation conflicts",
			},
			[]string{"driver_id"},
		),

		// Driver location metrics
		DriverLocationUpdatesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "driver_location_updates_total",
				Help: "Total number of driver location updates",
			},
			[]string{"driver_id"},
		),
		DriverLocationStaleTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "driver_location_stale_total",
				Help: "Total number of stale driver locations detected",
			},
			[]string{"driver_id"},
		),

		// Redis metrics
		RedisGeoQueryDurationSeconds: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "driver_redis_geo_query_duration_seconds",
				Help:    "Duration of Redis GEO queries in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		RedisLockContentionTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "driver_redis_lock_contention_total",
				Help: "Total number of Redis lock contentions",
			},
			[]string{"driver_id"},
		),

		// Kafka metrics
		KafkaPublishFailuresTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "driver_kafka_publish_failures_total",
				Help: "Total number of Kafka publish failures",
			},
			[]string{"topic"},
		),
		KafkaConsumerLag: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "driver_kafka_consumer_lag",
				Help: "Kafka consumer lag in messages",
			},
			[]string{"topic", "partition"},
		),

		// NATS metrics
		NATPublishFailuresTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "driver_nats_publish_failures_total",
				Help: "Total number of NATS publish failures",
			},
			[]string{"subject"},
		),

		// gRPC metrics
		GrpcRequestDurationSeconds: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "driver_grpc_request_duration_seconds",
				Help:    "Duration of gRPC requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "status"},
		),

		// Driver state metrics
		DriverStateGauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "driver_driver_state",
				Help: "Current state of drivers (1=OFFLINE, 2=AVAILABLE, 3=BUSY, 4=SUSPENDED, 5=BLOCKED)",
			},
			[]string{"driver_id"},
		),
		DriverCountGauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "driver_driver_count",
				Help: "Number of drivers per state",
			},
			[]string{"status"},
		),

		// Assignment state metrics
		AssignmentStateGauge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "driver_assignment_state",
				Help: "Current state of assignments (1=NONE, 2=OFFERED, 3=ACCEPTED, 4=ACTIVE, 5=COMPLETED, 6=REJECTED, 7=EXPIRED, 8=CANCELLED)",
			},
			[]string{"assignment_id", "driver_id"},
		),
	}
}

// RecordDispatchRequest increments dispatch request counter.
func (m *Metrics) RecordDispatchRequest(status string) {
	m.DispatchRequestsTotal.WithLabelValues(status).Inc()
}

// RecordDispatchSuccess increments successful dispatch counter.
func (m *Metrics) RecordDispatchSuccess(driverID string) {
	m.DispatchSuccessTotal.WithLabelValues(driverID).Inc()
}

// RecordDispatchFailure increments failed dispatch counter.
func (m *Metrics) RecordDispatchFailure(reason string) {
	m.DispatchFailureTotal.WithLabelValues(reason).Inc()
}

// RecordDispatchDuration records dispatch operation duration.
func (m *Metrics) RecordDispatchDuration(operation string, durationSeconds float64) {
	m.DispatchDurationSeconds.WithLabelValues(operation).Observe(durationSeconds)
}

// RecordAssignmentCreated increments assignment counter.
func (m *Metrics) RecordAssignmentCreated(status string) {
	m.DriverAssignmentTotal.WithLabelValues(status).Inc()
}

// RecordAssignmentAccepted increments accepted assignment counter.
func (m *Metrics) RecordAssignmentAccepted(driverID string) {
	m.DriverAssignmentAcceptTotal.WithLabelValues(driverID).Inc()
}

// RecordAssignmentRejected increments rejected assignment counter.
func (m *Metrics) RecordAssignmentRejected(driverID, reason string) {
	m.DriverAssignmentRejectTotal.WithLabelValues(driverID, reason).Inc()
}

// RecordAssignmentExpired increments expired assignment counter.
func (m *Metrics) RecordAssignmentExpired(driverID string) {
	m.DriverAssignmentExpireTotal.WithLabelValues(driverID).Inc()
}

// RecordReservationConflict increments reservation conflict counter.
func (m *Metrics) RecordReservationConflict(driverID string) {
	m.DriverReservationConflictTotal.WithLabelValues(driverID).Inc()
}

// RecordLocationUpdate increments location update counter.
func (m *Metrics) RecordLocationUpdate(driverID string) {
	m.DriverLocationUpdatesTotal.WithLabelValues(driverID).Inc()
}

// RecordStaleLocation increments stale location counter.
func (m *Metrics) RecordStaleLocation(driverID string) {
	m.DriverLocationStaleTotal.WithLabelValues(driverID).Inc()
}

// RecordGeoQueryDuration records Redis GEO query duration.
func (m *Metrics) RecordGeoQueryDuration(operation string, durationSeconds float64) {
	m.RedisGeoQueryDurationSeconds.WithLabelValues(operation).Observe(durationSeconds)
}

// RecordLockContention increments lock contention counter.
func (m *Metrics) RecordLockContention(driverID string) {
	m.RedisLockContentionTotal.WithLabelValues(driverID).Inc()
}

// RecordKafkaPublishFailure increments Kafka publish failure counter.
func (m *Metrics) RecordKafkaPublishFailure(topic string) {
	m.KafkaPublishFailuresTotal.WithLabelValues(topic).Inc()
}

// SetKafkaConsumerLag sets Kafka consumer lag gauge.
func (m *Metrics) SetKafkaConsumerLag(topic string, partition int, lag float64) {
	m.KafkaConsumerLag.WithLabelValues(topic, string(rune(partition))).Set(lag)
}

// RecordNATPublishFailure increments NATS publish failure counter.
func (m *Metrics) RecordNATPublishFailure(subject string) {
	m.NATPublishFailuresTotal.WithLabelValues(subject).Inc()
}

// RecordGrpcRequestDuration records gRPC request duration.
func (m *Metrics) RecordGrpcRequestDuration(method, status string, durationSeconds float64) {
	m.GrpcRequestDurationSeconds.WithLabelValues(method, status).Observe(durationSeconds)
}

// SetDriverState sets driver state gauge.
func (m *Metrics) SetDriverState(driverID string, state float64) {
	m.DriverStateGauge.WithLabelValues(driverID).Set(state)
}

// SetDriverCount sets driver count per status gauge.
func (m *Metrics) SetDriverCount(status string, count float64) {
	m.DriverCountGauge.WithLabelValues(status).Set(count)
}

// SetAssignmentState sets assignment state gauge.
func (m *Metrics) SetAssignmentState(assignmentID, driverID string, state float64) {
	m.AssignmentStateGauge.WithLabelValues(assignmentID, driverID).Set(state)
}