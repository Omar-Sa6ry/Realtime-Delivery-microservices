package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	pkgnats "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/nats"
)

type RealtimePublisher struct {
	client *pkgnats.NatsClient
}

func NewRealtimePublisher(natsURL string) (*RealtimePublisher, error) {
	client, err := pkgnats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("realtime_publisher: %w", err)
	}
	return &RealtimePublisher{client: client}, nil
}

type PaymentStatusUpdate struct {
	PaymentID  string    `json:"paymentId"`
	DeliveryID string    `json:"deliveryId"`
	UserID     string    `json:"userId"`
	Status     string    `json:"status"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (p *RealtimePublisher) PublishPaymentStatusUpdated(ctx context.Context, payload []byte) error {
	var update PaymentStatusUpdate
	if err := json.Unmarshal(payload, &update); err != nil {
		return fmt.Errorf("realtime_publisher: unmarshal payload: %w", err)
	}

	if err := p.client.PublishNestJS("payment.status.updated", update); err != nil {
		slog.Warn("realtime_publisher: NATS publish failed (non-critical)",
			"paymentId", update.PaymentID,
			"status", update.Status,
			"error", err,
		)
		return nil
	}

	slog.Debug("realtime_publisher: published payment.status.updated",
		"paymentId", update.PaymentID,
		"status", update.Status,
	)
	return nil
}

func (p *RealtimePublisher) PublishDirect(pattern string, data interface{}) error {
	return p.client.PublishNestJS(pattern, data)
}

func (p *RealtimePublisher) Close() {
	p.client.Close()
}
