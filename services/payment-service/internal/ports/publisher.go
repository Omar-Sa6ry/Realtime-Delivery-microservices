package ports

import "context"
type EventPublisherService interface {
	Publish(ctx context.Context, topic, key string, payload []byte) error
	Close() error
}
