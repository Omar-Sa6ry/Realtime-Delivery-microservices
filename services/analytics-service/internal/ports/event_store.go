package ports

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
)

type IdempotencyStore interface {
	Exists(ctx context.Context, eventID string) (bool, error)
	Save(ctx context.Context, record *domain.IdempotencyRecord) error
	LoadCheckpoint(ctx context.Context, topic string, partition int32) (*domain.IngestionCheckpoint, error)
	SaveCheckpoint(ctx context.Context, checkpoint *domain.IngestionCheckpoint) error
	Close() error
}
