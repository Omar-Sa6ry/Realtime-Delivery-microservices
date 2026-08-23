package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	ServiceName    string
	JaegerEndpoint string
	OTLPEndpoint   string
	Environment    string
	SampleRatio    float64
}

// InitTracer initializes an OpenTelemetry provider with resource metadata,
// parent-based sampling, and W3C context propagation. Exporter delivery is
// deployment-specific and can be attached by the runtime collector layer.
func InitTracer(ctx context.Context, cfg Config) (*sdktrace.TracerProvider, error) {
	if cfg.SampleRatio < 0 || cfg.SampleRatio > 1 {
		return nil, fmt.Errorf("invalid trace sample ratio %v", cfg.SampleRatio)
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironment(cfg.Environment),
			attribute.String("host.name", os.Getenv("HOSTNAME")),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}

	options := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	}
	if cfg.OTLPEndpoint != "" {
		exporter, exportErr := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
			otlptracegrpc.WithInsecure(),
		)
		if exportErr != nil {
			return nil, fmt.Errorf("create OTLP exporter: %w", exportErr)
		}
		options = append(options, sdktrace.WithBatcher(exporter))
		slog.Info("OTLP trace exporter configured", "endpoint", cfg.OTLPEndpoint)
	} else if cfg.JaegerEndpoint != "" {
		slog.Warn("Jaeger endpoint configured without OTLP; configure an OTLP collector endpoint")
	}
	tp := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return tp, nil
}

func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func ExtractTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

func InjectTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}
