package nats

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewRealtimePublisher(t *testing.T) {
	p := NewRealtimePublisher(nil, discardLogger())
	if p == nil {
		t.Fatal("NewRealtimePublisher returned nil")
	}
	if p.client != nil {
		t.Fatal("client must be stored as-is (nil in this test)")
	}

	var _ ports.RealtimePublisher = p
}

func TestPublishProgressMarshalError(t *testing.T) {
	p := NewRealtimePublisher(nil, discardLogger())

	err := p.PublishProgress(context.Background(), "media.progress", make(chan int))
	if err == nil {
		t.Fatal("expected marshal error for unmarshalable payload")
	}
	if !strings.Contains(err.Error(), "marshal realtime payload") {
		t.Fatalf("error = %v, want marshal realtime payload context", err)
	}
}
