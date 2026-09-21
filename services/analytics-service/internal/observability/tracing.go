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

type Tracer struct {
	provider    *sdktrace.TracerProvider
	tracer      trace.Tracer
	serviceName string
}

type TracerConfig struct {
	ServiceName    string
	OTLPEndpoint   string
	SamplingRate   float64
	EnableInsecure bool
}

func NewTracer(config TracerConfig) (*Tracer, error) {
	if config.ServiceName == "" {
		config.ServiceName = "analytics-service"
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

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(config.SamplingRate))),
	}
	if config.OTLPEndpoint != "" {
		exporterOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(config.OTLPEndpoint)}
		if config.EnableInsecure {
			exporterOpts = append(exporterOpts, otlptracegrpc.WithInsecure())
		}
		exporter, err := otlptracegrpc.New(context.Background(), exporterOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}

	provider := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Tracer{
		provider:    provider,
		tracer:      provider.Tracer(config.ServiceName),
		serviceName: config.ServiceName,
	}, nil
}

func (t *Tracer) Tracer() trace.Tracer {
	return t.tracer
}

func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

func (t *Tracer) StartConsumeSpan(ctx context.Context, topic, eventType, eventID string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, "analytics.consume",
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", topic),
			attribute.String("analytics.event_type", eventType),
			attribute.String("analytics.event_id", eventID),
		),
	)
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
	sc := span.SpanContext()
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}

func RecordError(ctx context.Context, err error, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil && err != nil {
		span.RecordError(err, trace.WithAttributes(attrs...))
	}
}

type Operation struct {
	span  trace.Span
	ctx   context.Context
	name  string
	start time.Time
}

func StartOperation(t *Tracer, ctx context.Context, name string, attrs ...attribute.KeyValue) *Operation {
	ctx, span := t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
	return &Operation{span: span, ctx: ctx, name: name, start: time.Now()}
}

func (o *Operation) Finish(err error) {
	if o.span != nil {
		if err != nil {
			o.span.RecordError(err)
			o.span.SetStatus(2, err.Error())
		} else {
			o.span.SetStatus(1, "success")
		}
		o.span.End()
	}
}

// Context returns the span context.
func (o *Operation) Context() context.Context {
	return o.ctx
}
