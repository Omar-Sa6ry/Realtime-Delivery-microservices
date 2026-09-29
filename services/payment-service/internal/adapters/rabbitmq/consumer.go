package rabbitmq

import (
	"context"

	shared "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type DispatchResponseHandler func(ctx context.Context, env *shared.EventEnvelope) error

type RabbitMQConsumer struct {
	consumer *shared.Consumer
	handler  DispatchResponseHandler
}

func NewRabbitMQConsumer(conn *shared.Connection, handler DispatchResponseHandler) *RabbitMQConsumer {
	c := shared.NewConsumer(shared.ConsumerConfig{
		Conn:        conn,
		Queue:       shared.QueueDispatchResponses,
		Exchange:    shared.ExchangeDispatch,
		RoutingKeys: []string{shared.DispatchRoutingKeyResponse},
		Prefetch:    shared.PrefetchFor(shared.QueueDispatchResponses),
	})
	return &RabbitMQConsumer{consumer: c, handler: handler}
}

// Run consumes until ctx is cancelled using the handler given at construction.
func (c *RabbitMQConsumer) Run(ctx context.Context) error {
	return c.RunWithHandler(ctx, c.handler)
}

// RunWithHandler consumes until ctx is cancelled using an explicit handler.
func (c *RabbitMQConsumer) RunWithHandler(ctx context.Context, handler DispatchResponseHandler) error {
	if handler == nil {
		handler = func(_ context.Context, _ *shared.EventEnvelope) error { return nil }
	}
	return c.consumer.Run(ctx, func(ctx context.Context, env *shared.EventEnvelope, _ amqp.Delivery) error {
		return handler(ctx, env)
	})
}
