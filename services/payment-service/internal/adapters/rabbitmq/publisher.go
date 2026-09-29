package rabbitmq

import (
	"context"
	"fmt"

	shared "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
)

type RabbitMQPublisher struct {
	pub *shared.Publisher
}

func NewRabbitMQPublisher(conn *shared.Connection) *RabbitMQPublisher {
	return &RabbitMQPublisher{pub: shared.NewPublisher(conn)}
}

func EnsurePaymentTopology(conn *shared.Connection) error {
	return conn.EnsureTopology([]shared.TopologyBinding{
		{Exchange: shared.ExchangePayments, Queue: shared.QueuePaymentsAuth, RoutingKey: shared.PaymentsRoutingKeyAuthorized},
		{Exchange: shared.ExchangePayments, Queue: shared.QueuePaymentsFailed, RoutingKey: shared.PaymentsRoutingKeyFailed},
		{Exchange: shared.ExchangePayments, Queue: shared.QueuePaymentsRefunded, RoutingKey: shared.PaymentsRoutingKeyRefunded},
		{Exchange: shared.ExchangeDispatch, Queue: shared.QueueDispatchResponses, RoutingKey: shared.DispatchRoutingKeyResponse},
	})
}

// PublishAuthorized publishes a payment.authorized event.
func (p *RabbitMQPublisher) PublishAuthorized(ctx context.Context, payload interface{}, opts *shared.PublishOptions) error {
	if p == nil || p.pub == nil {
		return fmt.Errorf("payment RabbitMQPublisher is not initialized")
	}
	return p.pub.Publish(ctx, shared.ExchangePayments, shared.PaymentsRoutingKeyAuthorized, "payment.authorized", payload, opts)
}

// PublishFailed publishes a payment.failed event.
func (p *RabbitMQPublisher) PublishFailed(ctx context.Context, payload interface{}, opts *shared.PublishOptions) error {
	if p == nil || p.pub == nil {
		return fmt.Errorf("payment RabbitMQPublisher is not initialized")
	}
	return p.pub.Publish(ctx, shared.ExchangePayments, shared.PaymentsRoutingKeyFailed, "payment.failed", payload, opts)
}

// PublishRefunded publishes a payment.refunded event.
func (p *RabbitMQPublisher) PublishRefunded(ctx context.Context, payload interface{}, opts *shared.PublishOptions) error {
	if p == nil || p.pub == nil {
		return fmt.Errorf("payment RabbitMQPublisher is not initialized")
	}
	return p.pub.Publish(ctx, shared.ExchangePayments, shared.PaymentsRoutingKeyRefunded, "payment.refunded", payload, opts)
}

// Close closes the underlying publisher channel (not the shared connection).
func (p *RabbitMQPublisher) Close() error {
	if p == nil || p.pub == nil {
		return nil
	}
	return p.pub.Close()
}
