package rabbitmq

import (
	"context"

	shared "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type DispatchRequestHandler func(ctx context.Context, env *shared.EventEnvelope) error

type DispatchConsumer struct {
	consumer *shared.Consumer
	handler  DispatchRequestHandler
}

// NewDispatchConsumer builds a consumer for dispatch.requests.queue.
func NewDispatchConsumer(conn *shared.Connection, handler DispatchRequestHandler) *DispatchConsumer {
	c := shared.NewConsumer(shared.ConsumerConfig{
		Conn:        conn,
		Queue:       shared.QueueDispatchRequests,
		Exchange:    shared.ExchangeDispatch,
		RoutingKeys: []string{shared.DispatchRoutingKeyRequest},
		Prefetch:    shared.PrefetchFor(shared.QueueDispatchRequests),
	})
	return &DispatchConsumer{consumer: c, handler: handler}
}

// Start consumes until ctx is cancelled using the handler given at construction.
func (c *DispatchConsumer) Start(ctx context.Context) error {
	return c.StartWithHandler(ctx, c.handler)
}

// StartWithHandler consumes until ctx is cancelled using an explicit handler.
func (c *DispatchConsumer) StartWithHandler(ctx context.Context, handler DispatchRequestHandler) error {
	if handler == nil {
		handler = func(_ context.Context, _ *shared.EventEnvelope) error { return nil }
	}
	return c.consumer.Run(ctx, func(ctx context.Context, env *shared.EventEnvelope, _ amqp.Delivery) error {
		return handler(ctx, env)
	})
}
