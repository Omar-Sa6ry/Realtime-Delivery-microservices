package nats

import (
	"context"
	"time"
	"github.com/nats-io/nats.go"
)

// NATSSubscriber subscribes to NATS subjects for location updates and assignment events.
type NATSSubscriber struct {
	subscriber *nats.Subscription
	nc         *nats.Conn
}

// NewNATSSubscriber creates a new NATSSubscriber.
func NewNATSSubscriber(nc *nats.Conn, subject string) (*NATSSubscriber, error) {
	sub, err := nc.Subscribe(subject, func(m *nats.Msg) {
		// Message received - handle here
	})
	if err != nil {
		return nil, err
	}
	return &NATSSubscriber{subscriber: sub, nc: nc}, nil
}

// NextMessage returns the next message from the subscription.
func (s *NATSSubscriber) NextMessage(ctx context.Context, timeout time.Duration) (*nats.Msg, error) {
	return s.subscriber.NextMsg(timeout)
}

// Close closes the subscription.
func (s *NATSSubscriber) Close() error {
	return s.subscriber.Unsubscribe()
}