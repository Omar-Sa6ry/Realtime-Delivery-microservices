package observability

import (
	"context"
	"fmt"
	"sync"
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

type Tracer struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	serviceName string
}

type TracerConfig struct {
	ServiceName    string
	OTLPEndpoint   string
	SamplingRate   float64
	EnableInsecure bool
}

// NewTracer creates a new OpenTelemetry tracer with an OTLP/gRPC exporter and
// installs it as the process-wide tracer. Returns nil when no endpoint is
// configured so the service can run untraced.
func NewTracer(config TracerConfig) (*Tracer, error) {
	if config.ServiceName == "" {
		config.ServiceName = "payment-service"
	}
	if config.OTLPEndpoint == "" {
		return nil, nil
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

	otel.SetTracerProvider(provider)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer := provider.Tracer(config.ServiceName)

	current := &Tracer{
		provider:    provider,
		tracer:      tracer,
		serviceName: config.ServiceName,
	}
	setCurrentTracer(current)
	return current, nil
}

var (
	currentMu sync.RWMutex
	currentTr *Tracer
)

func setCurrentTracer(t *Tracer) {
	currentMu.Lock()
	defer currentMu.Unlock()
	currentTr = t
}

func CurrentTracer() *Tracer {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return currentTr
}

func StartOperation(ctx context.Context, operation, paymentID string) *PaymentOperation {
	t := CurrentTracer()
	if t == nil {
		return &PaymentOperation{ctx: ctx, name: operation, start: time.Now()}
	}
	return StartPaymentOperation(t, ctx, operation, paymentID)
}

func (t *Tracer) Tracer() trace.Tracer {
	return t.tracer
}

func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

func (t *Tracer) StartSpanWithAttributes(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func (t *Tracer) Shutdown(ctx context.Context) error {
	if t.provider != nil {
		return t.provider.Shutdown(ctx)
	}
	return nil
}

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

func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

func RecordError(ctx context.Context, err error, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil && err != nil {
		span.RecordError(err, trace.WithAttributes(attrs...))
	}
}

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

func WithPaymentAttributes(ctx context.Context, paymentID, deliveryID, userID string, amount int64, currency string) context.Context {
	return context.WithValue(ctx, paymentAttributesKey{}, paymentAttrs{
		paymentID:  paymentID,
		deliveryID: deliveryID,
		userID:     userID,
		amount:     amount,
		currency:   currency,
	})
}

func GetPaymentAttributes(ctx context.Context) (string, string, string, int64, string) {
	val := ctx.Value(paymentAttributesKey{})
	if attrs, ok := val.(paymentAttrs); ok {
		return attrs.paymentID, attrs.deliveryID, attrs.userID, attrs.amount, attrs.currency
	}
	return "", "", "", 0, ""
}

type paymentAttributesKey struct{}

type paymentAttrs struct {
	paymentID  string
	deliveryID string
	userID     string
	amount     int64
	currency   string
}

type PaymentOperation struct {
	tracer *Tracer
	ctx    context.Context
	span   trace.Span
	name   string
	start  time.Time
}

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

func (p *PaymentOperation) AddAttribute(key string, value interface{}) {
	if p.span != nil {
		p.span.SetAttributes(attribute.String(p.name+"."+key, fmt.Sprintf("%v", value)))
	}
}

func (p *PaymentOperation) Context() context.Context {
	return p.ctx
}

func GetTracerFromContext(ctx context.Context) trace.Tracer {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return nil
	}
	return span.TracerProvider().Tracer("")
}
