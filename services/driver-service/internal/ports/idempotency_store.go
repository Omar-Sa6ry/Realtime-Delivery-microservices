package ports

import "context"

// IdempotencyStore defines the interface for idempotency operations.
type IdempotencyStore interface {
	// CheckAndStore checks if a key exists and stores the result atomically.
	// Returns true if the key was stored (first time), false if it already existed.
	CheckAndStore(ctx context.Context, key string, result []byte, ttl int) (bool, error)

	// Exists checks if a key exists in the idempotency store.
	Exists(ctx context.Context, key string) (bool, error)

	// Delete removes an idempotency key.
	Delete(ctx context.Context, key string) error
}