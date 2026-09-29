package rabbitmq

import (
	"context"
	"fmt"
	"strings"

	shared "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
)

type MediaRabbitMQPublisher struct {
	pub *shared.Publisher
}

func NewMediaRabbitMQPublisher(conn *shared.Connection) *MediaRabbitMQPublisher {
	return &MediaRabbitMQPublisher{pub: shared.NewPublisher(conn)}
}

func EnsureMediaTopology(conn *shared.Connection) error {
	return conn.EnsureTopology([]shared.TopologyBinding{
		{Exchange: shared.ExchangeMedia, Queue: shared.QueueMediaUploaded, RoutingKey: shared.MediaRoutingKeyAll},
		{Exchange: shared.ExchangeMedia, Queue: shared.QueueMediaProcessed, RoutingKey: shared.MediaRoutingKeyAll},
		{Exchange: shared.ExchangeMedia, Queue: shared.QueueSearchIndex, RoutingKey: shared.MediaRoutingKeyAll},
	})
}

func routingKeyFor(base, mediaType string) string {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if mediaType == "" {
		return base
	}
	return base + "." + mediaType
}

func (p *MediaRabbitMQPublisher) PublishUploaded(ctx context.Context, mediaType string, payload interface{}, opts *shared.PublishOptions) error {
	if p == nil || p.pub == nil {
		return fmt.Errorf("media RabbitMQPublisher is not initialized")
	}
	rk := routingKeyFor(shared.MediaRoutingKeyUploaded, mediaType)
	return p.pub.Publish(ctx, shared.ExchangeMedia, rk, "media.uploaded", payload, opts)
}

func (p *MediaRabbitMQPublisher) PublishProcessed(ctx context.Context, mediaType string, payload interface{}, opts *shared.PublishOptions) error {
	if p == nil || p.pub == nil {
		return fmt.Errorf("media RabbitMQPublisher is not initialized")
	}
	rk := routingKeyFor(shared.MediaRoutingKeyProcessed, mediaType)
	return p.pub.Publish(ctx, shared.ExchangeMedia, rk, "media.processed", payload, opts)
}

// Close closes the underlying publisher channel (not the shared connection).
func (p *MediaRabbitMQPublisher) Close() error {
	if p == nil || p.pub == nil {
		return nil
	}
	return p.pub.Close()
}
