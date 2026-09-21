package ingestion

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

type Deduplicator struct {
	store ports.IdempotencyStore
}

func NewDeduplicator(store ports.IdempotencyStore) *Deduplicator {
	return &Deduplicator{store: store}
}

func (d *Deduplicator) IsDuplicate(ctx context.Context, eventID string) (bool, error) {
	return d.store.Exists(ctx, eventID)
}
