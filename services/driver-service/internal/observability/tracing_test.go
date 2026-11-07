package observability_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/observability"
)

func TestNewTracerEmptyEndpointReturnsNil(t *testing.T) {
	tests := []struct {
		name   string
		config observability.TracerConfig
	}{
		{"empty config", observability.TracerConfig{}},
		{"service name but no endpoint", observability.TracerConfig{ServiceName: "driver-service"}},
		{"endpoint omitted with sampling", observability.TracerConfig{ServiceName: "svc", SamplingRate: 0.5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer, err := observability.NewTracer(tt.config)
			if tracer != nil {
				t.Errorf("NewTracer() tracer = %v, want nil", tracer)
			}
			if err != nil {
				t.Errorf("NewTracer() err = %v, want nil", err)
			}
		})
	}
}

func TestNewTracerInvalidEndpointReturnsError(t *testing.T) {
	tracer, err := observability.NewTracer(observability.TracerConfig{
		ServiceName:  "driver-service",
		OTLPEndpoint: "%%%",
	})
	if tracer != nil {
		t.Errorf("NewTracer() tracer = %v, want nil on exporter error", tracer)
	}
	if err == nil {
		t.Fatalf("NewTracer() err = nil, want exporter creation error")
	}
	if !strings.Contains(err.Error(), "create OTLP exporter") {
		t.Errorf("NewTracer() err = %q, want it to mention create OTLP exporter", err.Error())
	}
}

func TestNewTracerWithEndpoint(t *testing.T) {
	tracer, err := observability.NewTracer(observability.TracerConfig{
		ServiceName:  "driver-service-test",
		OTLPEndpoint: "127.0.0.1:4317",
		SamplingRate: 0.5,
	})
	if err != nil {
		t.Fatalf("NewTracer() err = %v, want nil", err)
	}
	if tracer == nil {
		t.Fatalf("NewTracer() tracer = nil, want non-nil")
	}

	tel := tracer.Tracer()
	if tel == nil {
		t.Errorf("Tracer() returned nil")
	}

	// The tracer must be usable: starting a span exercises the provider.
	ctx, span := tel.Start(context.Background(), "test-span")
	span.End()
	if ctx == nil {
		t.Errorf("span context is nil")
	}

	if err := tracer.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown() err = %v, want nil", err)
	}
}

func TestNewTracerDefaultSamplingRate(t *testing.T) {
	// SamplingRate <= 0 must fall back to 1.0 without error.
	tracer, err := observability.NewTracer(observability.TracerConfig{
		ServiceName:  "",
		OTLPEndpoint: "127.0.0.1:4317",
		SamplingRate: -1,
	})
	if err != nil {
		t.Fatalf("NewTracer() err = %v, want nil", err)
	}
	if tracer == nil {
		t.Fatalf("NewTracer() tracer = nil, want non-nil")
	}
	if tracer.Tracer() == nil {
		t.Errorf("Tracer() returned nil")
	}
	if err := tracer.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown() err = %v, want nil", err)
	}
}

func TestShutdownNilReceiver(t *testing.T) {
	var tracer *observability.Tracer
	if err := tracer.Shutdown(context.Background()); err != nil {
		t.Errorf("nil receiver Shutdown() = %v, want nil", err)
	}
}
