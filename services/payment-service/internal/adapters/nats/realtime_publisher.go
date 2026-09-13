package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/realtime-delivery/payment-service/internal/ports"
)

// RealtimePublisher publishes payment status updates to NATS for real-time notifications.
type RealtimePublisher struct {
	nc     *nats.Conn
	subject string
}

// NewRealtimePublisher creates a new RealtimePublisher.
func NewRealtimePublisher(natsURL, subject string) (*RealtimePublisher, error) {
	nc, err := nats.Connect(natsURL,
		nats.ReconnectWait(1*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &RealtimePublisher{
		nc:      nc,
		subject: subject,
	}, nil
}

// PublishPaymentStatusUpdated publishes a payment status update to NATS.
func (p *RealtimePublisher) PublishPaymentStatusUpdated(ctx context.Context, payload []byte) error {
	subject := "payment.status.updated"
	return p.nc.Publish(subject, payload)
}

// Close closes the NATS connection.
func (p *RealtimePublisher) Close() error {
	if p.nc != nil {
		p.nc.Drain()
	}
	return nil
}

// Connect connects to NATS and returns a connection.
func Connect(natsURL string) (*nats.Conn, error) {
	nc, err := nats.Connect(natsURL,
		nats.ReconnectWait(1*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	return nc, nil
}

// PublishNestJS publishes a message using NestJS envelope format.
func PublishNestJS(nc *nats.Conn, subject string, data interface{}) error {
	// NestJS envelope format: { event: string, data: any, timestamp: number }
	// This matches the NestJS event pattern used by the Realtime Service
	return nc.Publish(subject, []byte(fmt.Sprintf(`{"event":"%s","data":%v,"timestamp":%d}`, "payment.status.updated", "{}", 0)))
}