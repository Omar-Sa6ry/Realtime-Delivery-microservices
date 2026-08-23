package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	sharednats "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/nats"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// RealtimePublisher publishes realtime progress events to NATS
type RealtimePublisher struct {
	client *sharednats.NatsClient
	logger *slog.Logger
}

// NewRealtimePublisher creates a new realtime publisher
func NewRealtimePublisher(client *sharednats.NatsClient, logger *slog.Logger) *RealtimePublisher {
	return &RealtimePublisher{
		client: client,
		logger: logger.With("component", "realtime-publisher"),
	}
}

// PublishProgress publishes a progress event to NATS for WebSocket fan-out
func (p *RealtimePublisher) PublishProgress(ctx context.Context, subject string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal realtime payload: %w", err)
	}

	if err := p.client.Publish(subject, data); err != nil {
		p.logger.Error("NATS publish failed", "subject", subject, "error", err)
		return err
	}

	p.logger.Debug("Published realtime progress", "subject", subject)
	return nil
}

// Compile-time check that RealtimePublisher implements ports.RealtimePublisher
var _ ports.RealtimePublisher = (*RealtimePublisher)(nil)