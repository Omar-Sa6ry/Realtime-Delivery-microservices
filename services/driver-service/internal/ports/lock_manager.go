package ports

import (
	"context"
	"time"
)

type LockManager interface {
	Acquire(ctx context.Context, resource string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, resource string, token string) error
}