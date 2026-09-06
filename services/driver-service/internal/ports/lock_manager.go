package ports

import (
	"context"
	"time"
)

// LockManager defines the interface for distributed locking using Redis.
type LockManager interface {
	Acquire(ctx context.Context, resource string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, resource string, token string) error
}