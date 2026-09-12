package ports

import (
	"context"

	"github.com/nats-io/nats.go"
)

// EventPublisher defines the interface for publishing domain events.
type EventPublisher interface {
	PublishPaymentCreated(ctx context.Context, eventType string, payload string) error
	PublishPaymentAuthorized(ctx context.Context, eventType string, payload string) error
	PublishPaymentCaptured(ctx context.Context, eventType string, payload string) error
	PublishPaymentCancelled(ctx context.Context, eventType string, payload string) error
	PublishPaymentRefunded(ctx context.Context, eventType string, payload string) error
	PublishPaymentFailed(ctx context.Context, eventType string, payload string) error
}

// NATSPublisher implements EventPublisher using NATS.
type NATSPublisher struct {
	nc         *nats.Conn
	subject    string
	producer   string
}

// NewNATSPublisher creates a new NATSPublisher.
func NewNATSPublisher(nc *nats.Conn, subject, producer string) *NATSPublisher {
	return &NATSPublisher{
		nc:       nc,
		subject:  subject,
		producer: producer,
	}
}

// PublishPaymentCreated publishes a PaymentCreated event.
func (p *NATSPublisher) PublishPaymentCreated(ctx context.Context, eventType string, payload string) error {
	return p.nc.Publish(p.subject+"."+eventType, []byte(payload))
}

// PublishPaymentAuthorized publishes a PaymentAuthorized event.
func (p *NATSPublisher) PublishPaymentAuthorized(ctx context.Context, eventType string, payload string) error {
	return p.nc.Publish(p.subject+"."+eventType, []byte(payload))
}

// PublishPaymentCaptured publishes a PaymentCaptured event.
func (p *NATSPublisher) PublishPaymentCaptured(ctx context.Context, eventType string, payload string) error {
	return p.nc.Publish(p.subject+"."+eventType, []byte(payload))
}

// PublishPaymentCancelled publishes a PaymentCancelled event.
func (p *NATSPublisher) PublishPaymentCancelled(ctx context.Context, eventType string, payload string) error {
	return p.nc.Publish(p.subject+"."+eventType, []byte(payload))
}

// PublishPaymentRefunded publishes a PaymentRefunded event.
func (p *NATSPublisher) PublishPaymentRefunded(ctx context.Context, eventType string, payload string) error {
	return p.nc.Publish(p.subject+"."+eventType, []byte(payload))
}

// PublishPaymentFailed publishes a PaymentFailed event.
func (p *NATSPublisher) PublishPaymentFailed(ctx context.Context, eventType string, payload string) error {
	return p.nc.Publish(p.subject+"."+eventType, []byte(payload))
}

// IdempotencyStore defines the interface for idempotency key storage.
type IdempotencyStore interface {
	// Store saves an idempotency key with its result.
	Store(key string, result string) error
	// Retrieve retrieves a previously stored result for a key.
	Retrieve(key string) (string, bool)
	// Delete removes a stored result.
	Delete(key string) error
	// Exists checks if a key exists.
	Exists(key string) (bool, error)
}