package observability

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracer holds the OpenTelemetry tracer provider and meter provider.
type Tracer struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *metric.MeterProvider
}

// InitTracing initializes OpenTelemetry tracing and metrics.
func InitTracing(serviceName, jaegerEndpoint string) (*Tracer, error) {
	// Create resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, err
	}

	// Initialize Jaeger trace exporter
	traceExporter, err := jaeger.New(jaeger.WithAgentEndpoint(jaeger.WithAgentHost(jaegerEndpoint)))
	if err != nil {
		return nil, err
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	// Initialize Prometheus metrics exporter
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	// Create meter provider
	mp := metric.NewMeterProvider(
		metric.WithReader(promExporter),
		metric.WithResource(res),
	)

	// Set global providers
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Printf("OpenTelemetry initialized for service: %s", serviceName)

	return &Tracer{
		tracerProvider: tp,
		meterProvider:  mp,
	}, nil
}

// InitTracingFromEnv initializes tracing from environment variables.
func InitTracingFromEnv(serviceName string) (*Tracer, error) {
	jaegerEndpoint := os.Getenv("JAEGER_ENDPOINT")
	if jaegerEndpoint == "" {
		jaegerEndpoint = "jaeger:6831"
	}
	return InitTracing(serviceName, jaegerEndpoint)
}

// Shutdown shuts down the tracer and meter providers.
func (t *Tracer) Shutdown(ctx context.Context) error {
	var errs []error

	if t.tracerProvider != nil {
		if err := t.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if t.meterProvider != nil {
		if err := t.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Tracer returns the OpenTelemetry tracer.
func (t *Tracer) Tracer() trace.Tracer {
	return t.tracerProvider.Tracer("driver-service")
}

// StartSpan starts a new span with the given name and options.
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.Tracer().Start(ctx, name, opts...)
}

// InjectTraceContext injects trace context into carrier.
func (t *Tracer) InjectTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

// ExtractTraceContext extracts trace context from carrier.
func (t *Tracer) ExtractTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// SpanAttributes holds common span attributes for the driver service.
type SpanAttributes struct {
	DriverID      string
	DeliveryID    string
	AssignmentID  string
	CorrelationID string
	CausationID   string
	Operation     string
	Status        string
	Error         string
}

// ToAttributes converts SpanAttributes to OpenTelemetry attributes.
func (s *SpanAttributes) ToAttributes() []attribute.KeyValue {
	var attrs []attribute.KeyValue
	if s.DriverID != "" {
		attrs = append(attrs, semconv.NetPeerNameKey.String(s.DriverID))
	}
	if s.DeliveryID != "" {
		attrs = append(attrs, semconv.MessagingMessageIDKey.String(s.DeliveryID))
	}
	if s.AssignmentID != "" {
		attrs = append(attrs, attribute.String("driver.assignment_id", s.AssignmentID))
	}
	if s.CorrelationID != "" {
		attrs = append(attrs, attribute.String("trace_id", s.CorrelationID))
	}
	if s.CausationID != "" {
		attrs = append(attrs, attribute.String("causation_id", s.CausationID))
	}
	if s.Operation != "" {
		attrs = append(attrs, attribute.String("driver.operation", s.Operation))
	}
	if s.Status != "" {
		attrs = append(attrs, attribute.String("driver.status", s.Status))
	}
	if s.Error != "" {
		attrs = append(attrs, attribute.String("error", s.Error))
	}
	return attrs
}

// StartDriverSpan starts a span for driver operations.
func (t *Tracer) StartDriverSpan(ctx context.Context, operation, driverID string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	spanAttrs := []attribute.KeyValue{
		attribute.String("driver.id", driverID),
		attribute.String("driver.operation", operation),
	}
	spanAttrs = append(spanAttrs, attrs...)
	return t.StartSpan(ctx, "driver."+operation, trace.WithAttributes(spanAttrs...))
}

// StartAssignmentSpan starts a span for assignment operations.
func (t *Tracer) StartAssignmentSpan(ctx context.Context, operation, assignmentID, driverID, deliveryID string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	spanAttrs := []attribute.KeyValue{
		attribute.String("driver.assignment_id", assignmentID),
		attribute.String("driver.id", driverID),
		attribute.String("delivery.id", deliveryID),
		attribute.String("driver.operation", operation),
	}
	spanAttrs = append(spanAttrs, attrs...)
	return t.StartSpan(ctx, "driver.assignment."+operation, trace.WithAttributes(spanAttrs...))
}

// StartDispatchSpan starts a span for dispatch operations.
func (t *Tracer) StartDispatchSpan(ctx context.Context, operation, deliveryID string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	spanAttrs := []attribute.KeyValue{
		attribute.String("delivery.id", deliveryID),
		attribute.String("driver.operation", operation),
	}
	spanAttrs = append(spanAttrs, attrs...)
	return t.StartSpan(ctx, "driver.dispatch."+operation, trace.WithAttributes(spanAttrs...))
}

// StartGrpcSpan starts a span for gRPC operations.
func (t *Tracer) StartGrpcSpan(ctx context.Context, method string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	spanAttrs := []attribute.KeyValue{
		attribute.String("rpc.method", method),
		attribute.String("rpc.system", "grpc"),
	}
	spanAttrs = append(spanAttrs, attrs...)
	return t.StartSpan(ctx, "grpc."+method, trace.WithAttributes(spanAttrs...))
}

// StartKafkaSpan starts a span for Kafka operations.
func (t *Tracer) StartKafkaSpan(ctx context.Context, operation, topic string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	spanAttrs := []attribute.KeyValue{
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", topic),
		attribute.String("messaging.operation", operation),
	}
	spanAttrs = append(spanAttrs, attrs...)
	return t.StartSpan(ctx, "kafka."+operation, trace.WithAttributes(spanAttrs...))
}

// StartNATSSpan starts a span for NATS operations.
func (t *Tracer) StartNATSSpan(ctx context.Context, operation, subject string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	spanAttrs := []attribute.KeyValue{
		attribute.String("messaging.system", "nats"),
		attribute.String("messaging.destination", subject),
		attribute.String("messaging.operation", operation),
	}
	spanAttrs = append(spanAttrs, attrs...)
	return t.StartSpan(ctx, "nats."+operation, trace.WithAttributes(spanAttrs...))
}

// StartRedisSpan starts a span for Redis operations.
func (t *Tracer) StartRedisSpan(ctx context.Context, operation string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	spanAttrs := []attribute.KeyValue{
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", operation),
	}
	spanAttrs = append(spanAttrs, attrs...)
	return t.StartSpan(ctx, "redis."+operation, trace.WithAttributes(spanAttrs...))
}

// StartMongoDBSpan starts a span for MongoDB operations.
func (t *Tracer) StartMongoDBSpan(ctx context.Context, operation, collection string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	spanAttrs := []attribute.KeyValue{
		attribute.String("db.system", "mongodb"),
		attribute.String("db.collection", collection),
		attribute.String("db.operation", operation),
	}
	spanAttrs = append(spanAttrs, attrs...)
	return t.StartSpan(ctx, "mongodb."+operation, trace.WithAttributes(spanAttrs...))
}