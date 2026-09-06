package ports

import (
	"time"
	"context"
)

// IdempotencyStore defines the interface for idempotency key management.
type IdempotencyStore interface {
	CheckAndStore(ctx context.Context, key string, result []byte, ttl time.Duration) (bool, error)
	Exists(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, key string) error
}