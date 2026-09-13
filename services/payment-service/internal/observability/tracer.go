package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracer wraps the OpenTelemetry tracer for payment service.
type Tracer struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	serviceName string
}

// TracerConfig holds configuration for the tracer.
type TracerConfig struct {
	ServiceName    string
	OTLPEndpoint   string
	SamplingRate   float64
	EnableInsecure bool
}

// NewTracer creates a new OpenTelemetry tracer with Jaeger/OTLP exporter.
func NewTracer(config TracerConfig) (*Tracer, error) {
	if config.ServiceName == "" {
		config.ServiceName = "payment-service"
	}

	// Create the OTLP exporter
	exporter, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithEndpoint(config.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service name
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(config.SamplingRate))),
	)

	// Set global tracer provider
	otel.SetTracerProvider(provider)

	// Set global propagator for W3C trace context
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer := provider.Tracer(config.ServiceName)

	return &Tracer{
		provider:    provider,
		tracer:      tracer,
		serviceName: config.ServiceName,
	}, nil
}

// Tracer returns the OpenTelemetry tracer.
func (t *Tracer) Tracer() trace.Tracer {
	return t.tracer
}

// StartSpan starts a new span with the given name and options.
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// StartSpanWithAttributes starts a new span with attributes.
func (t *Tracer) StartSpanWithAttributes(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// Shutdown gracefully shuts down the tracer provider.
func (t *Tracer) Shutdown(ctx context.Context) error {
	if t.provider != nil {
		return t.provider.Shutdown(ctx)
	}
	return nil
}

// ExtractTraceID extracts the trace ID from the context.
func ExtractTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return ""
	}
	spanContext := span.SpanContext()
	if !spanContext.IsValid() {
		return ""
	}
	return spanContext.TraceID().String()
}

// ExtractSpanID extracts the span ID from the context.
func ExtractSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return ""
	}
	spanContext := span.SpanContext()
	if !spanContext.IsValid() {
		return ""
	}
	return spanContext.SpanID().String()
}

// AddEvent adds an event to the current span.
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// RecordError records an error on the current span.
func RecordError(ctx context.Context, err error, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil && err != nil {
		span.RecordError(err, trace.WithAttributes(attrs...))
	}
}

// SetStatus sets the span status.
func SetStatus(ctx context.Context, code int, description string) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		if code == 0 {
			span.SetStatus(1, description) // StatusCodeOK = 1
		} else {
			span.SetStatus(2, description) // StatusCodeError = 2
		}
	}
}

// WithPaymentAttributes adds payment-specific attributes to the context.
func WithPaymentAttributes(ctx context.Context, paymentID, deliveryID, userID string, amount int64, currency string) context.Context {
	return context.WithValue(ctx, paymentAttributesKey{}, paymentAttrs{
		paymentID:  paymentID,
		deliveryID: deliveryID,
		userID:     userID,
		amount:     amount,
		currency:   currency,
	})
}

// GetPaymentAttributes extracts payment attributes from the context.
func GetPaymentAttributes(ctx context.Context) (string, string, string, int64, string) {
	val := ctx.Value(paymentAttributesKey{})
	if attrs, ok := val.(paymentAttrs); ok {
		return attrs.paymentID, attrs.deliveryID, attrs.userID, attrs.amount, attrs.currency
	}
	return "", "", "", 0, ""
}

// paymentAttributesKey is a context key for payment attributes.
type paymentAttributesKey struct{}

type paymentAttrs struct {
	paymentID  string
	deliveryID string
	userID     string
	amount     int64
	currency   string
}

// PaymentOperation represents a payment operation for tracing.
type PaymentOperation struct {
	tracer *Tracer
	ctx    context.Context
	span   trace.Span
	name   string
	start  time.Time
}

// StartPaymentOperation starts a new payment operation span.
func StartPaymentOperation(t *Tracer, ctx context.Context, operation string, paymentID string) *PaymentOperation {
	ctx, span := t.StartSpanWithAttributes(ctx, "payment."+operation,
		attribute.String("payment.operation", operation),
		attribute.String("payment.id", paymentID),
	)
	return &PaymentOperation{
		tracer: t,
		ctx:    ctx,
		span:   span,
		name:   operation,
		start:  time.Now(),
	}
}

// Finish finishes the payment operation span.
func (p *PaymentOperation) Finish(err error) {
	if p.span != nil {
		if err != nil {
			p.span.RecordError(err)
			p.span.SetStatus(2, err.Error()) // StatusCodeError = 2
		} else {
			p.span.SetStatus(1, "success") // StatusCodeOK = 1
		}
		p.span.End()
	}
}

// AddAttribute adds an attribute to the span.
func (p *PaymentOperation) AddAttribute(key string, value interface{}) {
	if p.span != nil {
		p.span.SetAttributes(attribute.String(p.name+"."+key, fmt.Sprintf("%v", value)))
	}
}

// Context returns the context with the span.
func (p *PaymentOperation) Context() context.Context {
	return p.ctx
}

// GetTracerFromContext extracts the tracer from the context.
func GetTracerFromContext(ctx context.Context) trace.Tracer {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return nil
	}
	return span.TracerProvider().Tracer("")
}