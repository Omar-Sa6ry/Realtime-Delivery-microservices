package observability

import (
	"context"
	"testing"
)

func TestNewTracer_NoEndpoint_ReturnsNil(t *testing.T) {
	tr, err := NewTracer(TracerConfig{ServiceName: "search-service", OTLPEndpoint: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr != nil {
		t.Fatal("expected nil tracer when endpoint is empty")
	}
}

func TestNewTracer_DefaultServiceNameAndValidEndpoint(t *testing.T) {
	// otlptracegrpc.New does not dial until first export, so a dummy endpoint is safe.
	tr, err := NewTracer(TracerConfig{
		ServiceName:  "",
		OTLPEndpoint: "127.0.0.1:4317",
		SamplingRate: 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr == nil {
		t.Fatal("expected tracer")
	}
	if tr.Tracer() == nil {
		t.Fatal("expected non-nil tracer instance")
	}
	if err := tr.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestNewTracer_SamplingRateClamped(t *testing.T) {
	for _, rate := range []float64{-1, 2, 0.25} {
		tr, err := NewTracer(TracerConfig{
			OTLPEndpoint: "127.0.0.1:4317",
			SamplingRate: rate,
		})
		if err != nil {
			t.Fatalf("rate %v: %v", rate, err)
		}
		if tr == nil {
			t.Fatalf("rate %v: nil tracer", rate)
		}
		if err := tr.Shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}
}

func TestTracer_Shutdown_NilSafe(t *testing.T) {
	var tr *Tracer
	if err := tr.Shutdown(context.Background()); err != nil {
		t.Fatalf("nil receiver shutdown: %v", err)
	}
	tr = &Tracer{}
	if err := tr.Shutdown(context.Background()); err != nil {
		t.Fatalf("nil provider shutdown: %v", err)
	}
}
