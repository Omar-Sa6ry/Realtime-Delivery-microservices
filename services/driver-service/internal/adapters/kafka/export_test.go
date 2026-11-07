package kafka

// Test-only hooks that expose unexported members to the external test package
// (package kafka_test). This file is only compiled during `go test` and does
// not affect production builds.

import (
	"context"

	kafkago "github.com/segmentio/kafka-go"
)

// ParseCoordinate exposes the unexported coordinate-parsing helper so the
// external tests can cover every type branch of the switch (JSON payloads can
// only ever produce float64/string/nil, so the remaining branches are
// unreachable through the message-handler path alone).
func ParseCoordinate(val interface{}, fallback float64) float64 {
	return parseCoordinate(val, fallback)
}

// HandleMessage exposes the unexported message handler so the external tests
// can drive rare branches directly without spinning up a consumer group.
func (c *DeliveryCreatedConsumer) HandleMessage(ctx context.Context, msg kafkago.Message) error {
	return c.handleMessage(ctx, msg)
}

// PublishEnvelope exposes the unexported envelope-publishing helper so the
// external tests can exercise the marshal-failure branch (every exported
// Publish* method only ever passes marshalable payloads).
func (a *EventPublisherAdapter) PublishEnvelope(ctx context.Context, eventType, key string, payload interface{}) error {
	return a.publish(ctx, eventType, key, payload)
}
