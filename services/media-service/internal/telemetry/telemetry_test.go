package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestInitTracerRejectsInvalidSampleRatio(t *testing.T) {
	for _, ratio := range []float64{-0.1, 1.5} {
		_, err := InitTracer(context.Background(), Config{
			ServiceName: "media-service",
			SampleRatio: ratio,
		})
		if err == nil {
			t.Fatalf("SampleRatio %v must be rejected", ratio)
		}
		if !strings.Contains(err.Error(), "invalid trace sample ratio") {
			t.Fatalf("error = %v, want invalid trace sample ratio", err)
		}
	}
}

func TestInitTracerWithoutEndpoints(t *testing.T) {
	tp, err := InitTracer(context.Background(), Config{
		ServiceName: "media-service",
		Environment: "test",
		SampleRatio: 1,
	})
	if err != nil {
		t.Fatalf("InitTracer: %v", err)
	}
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	if tp == nil {
		t.Fatal("tracer provider must not be nil")
	}
	if Tracer("test") == nil {
		t.Fatal("Tracer must return a non-nil tracer")
	}
}

func TestInitTracerWithOTLPEndpoint(t *testing.T) {
	tp, err := InitTracer(context.Background(), Config{
		ServiceName:  "media-service",
		Environment:  "test",
		SampleRatio:  1,
		OTLPEndpoint: "127.0.0.1:4317",
	})
	if err != nil {
		t.Fatalf("InitTracer: %v", err)
	}
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	if tp == nil {
		t.Fatal("tracer provider must not be nil")
	}
}

func TestInitTracerJaegerOnlyLogsWarning(t *testing.T) {
	prev := slog.Default()
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	tp, err := InitTracer(context.Background(), Config{
		ServiceName:    "media-service",
		Environment:    "test",
		SampleRatio:    1,
		JaegerEndpoint: "localhost:14268",
	})
	if err != nil {
		t.Fatalf("InitTracer: %v", err)
	}
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	if tp == nil {
		t.Fatal("tracer provider must not be nil")
	}
	if !strings.Contains(buf.String(), "Jaeger endpoint configured without OTLP") {
		t.Fatalf("slog output = %q, want Jaeger warning", buf.String())
	}
}

func TestInitTracerOTLPExporterError(t *testing.T) {
	tp, err := InitTracer(context.Background(), Config{
		ServiceName:  "media-service",
		Environment:  "test",
		SampleRatio:  1,
		OTLPEndpoint: "\x00",
	})
	if err != nil {
		if tp != nil {
			_ = tp.Shutdown(context.Background())
		}
		if !strings.Contains(err.Error(), "create OTLP exporter") {
			t.Fatalf("error = %v, want create OTLP exporter context", err)
		}
		return
	}
	_ = tp.Shutdown(context.Background())
	t.Skip("otlptracegrpc accepted an invalid endpoint; exporter error branch not reachable in this environment")
}

func TestPropagationRoundTrip(t *testing.T) {
	tp, err := InitTracer(context.Background(), Config{
		ServiceName: "media-service",
		Environment: "test",
		SampleRatio: 1,
	})
	if err != nil {
		t.Fatalf("InitTracer: %v", err)
	}
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	ctx, span := Tracer("test").Start(context.Background(), "upload")
	defer span.End()

	carrier := propagation.MapCarrier{}
	InjectTraceContext(ctx, carrier)
	if carrier.Get("traceparent") == "" {
		t.Fatal("traceparent header must be injected")
	}

	extracted := ExtractTraceContext(context.Background(), carrier)
	got := trace.SpanFromContext(extracted).SpanContext()
	if got.TraceID() != span.SpanContext().TraceID() {
		t.Fatalf("trace id = %v, want %v", got.TraceID(), span.SpanContext().TraceID())
	}
}

func TestTracerProviderShutdown(t *testing.T) {
	tp, err := InitTracer(context.Background(), Config{
		ServiceName: "media-service",
		Environment: "test",
		SampleRatio: 1,
	})
	if err != nil {
		t.Fatalf("InitTracer: %v", err)
	}
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}
