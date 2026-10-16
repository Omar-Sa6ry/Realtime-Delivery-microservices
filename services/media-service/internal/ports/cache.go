package ports

import (
	"context"
	"time"
)

type IdempotencyResult struct {
	Exists bool
	CachedResponse []byte
}

type LockToken string

type Cache interface {
	CheckAndStoreIdempotency(ctx context.Context, userID, key string, response []byte, ttl time.Duration) (*IdempotencyResult, error)
	AcquireLock(ctx context.Context, resource string, ttl time.Duration) (token LockToken, acquired bool, err error)
	ReleaseLock(ctx context.Context, resource string, token LockToken) error
	SetUploadProgress(ctx context.Context, uploadID string, percent int, ttl time.Duration) error
	GetUploadProgress(ctx context.Context, uploadID string) (percent int, found bool, err error)
	SetProcessingProgress(ctx context.Context, mediaID string, percent int, ttl time.Duration) error
	GetProcessingProgress(ctx context.Context, mediaID string) (percent int, found bool, err error)
	DeleteKey(ctx context.Context, key string) error
}
