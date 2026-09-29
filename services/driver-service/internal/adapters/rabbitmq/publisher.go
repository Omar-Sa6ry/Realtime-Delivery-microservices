package rabbitmq

import (
	"context"
	"fmt"

	shared "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
)

type RabbitMQPublisher struct {
	pub *shared.Publisher
}

// NewRabbitMQPublisher creates a driver RabbitMQ publisher bound to conn.
func NewRabbitMQPublisher(conn *shared.Connection) *RabbitMQPublisher {
	return &RabbitMQPublisher{pub: shared.NewPublisher(conn)}
}

func EnsureDriverTopology(conn *shared.Connection) error {
	return conn.EnsureTopology([]shared.TopologyBinding{
		{Exchange: shared.ExchangeDispatch, Queue: shared.QueueDispatchRequests, RoutingKey: shared.DispatchRoutingKeyRequest},
		{Exchange: shared.ExchangeDispatch, Queue: shared.QueueDispatchResponses, RoutingKey: shared.DispatchRoutingKeyResponse},
		{Exchange: shared.ExchangeDrivers, Queue: shared.QueueDriversEvents, RoutingKey: shared.DriversRoutingKeyAll},
	})
}

func (p *RabbitMQPublisher) PublishDispatchResponse(ctx context.Context, payload interface{}, opts *shared.PublishOptions) error {
	if p == nil || p.pub == nil {
		return fmt.Errorf("driver RabbitMQPublisher is not initialized")
	}
	return p.pub.Publish(ctx, shared.ExchangeDispatch, shared.DispatchRoutingKeyResponse, "dispatch.response", payload, opts)
}

func (p *RabbitMQPublisher) PublishDriverEvent(ctx context.Context, routingKey, eventType string, payload interface{}, opts *shared.PublishOptions) error {
	if p == nil || p.pub == nil {
		return fmt.Errorf("driver RabbitMQPublisher is not initialized")
	}
	if routingKey == "" {
		routingKey = shared.DriversRoutingKeyAll
	}
	if eventType == "" {
		eventType = routingKey
	}
	return p.pub.Publish(ctx, shared.ExchangeDrivers, routingKey, eventType, payload, opts)
}

// Close closes the underlying publisher channel (not the shared connection).
func (p *RabbitMQPublisher) Close() error {
	if p == nil || p.pub == nil {
		return nil
	}
	return p.pub.Close()
}
