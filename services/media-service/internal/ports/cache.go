package ports

import (
	"context"
	"time"
)

// IdempotencyResult holds the outcome of an idempotency check.
type IdempotencyResult struct {
	// Exists is true when the idempotency key was already stored (duplicate request).
	Exists bool
	// CachedResponse contains the previously cached response body (if any).
	CachedResponse []byte
}

// LockToken is an opaque token that must be presented when releasing a distributed lock.
type LockToken string

// Cache abstracts all Redis operations used by the media service.
type Cache interface {
	// CheckAndStoreIdempotency atomically checks whether an idempotency key exists
	// and stores it if not. Returns Exists=true if the request is a duplicate.
	// The key expires after ttl. The response is stored alongside the key.
	CheckAndStoreIdempotency(ctx context.Context, userID, key string, response []byte, ttl time.Duration) (*IdempotencyResult, error)

	// AcquireLock attempts to acquire a named distributed lock using SET NX PX.
	// Returns a LockToken that must be presented on release, and acquired=false if unavailable.
	AcquireLock(ctx context.Context, resource string, ttl time.Duration) (token LockToken, acquired bool, err error)

	// ReleaseLock releases a previously acquired lock using a Lua script for atomicity.
	ReleaseLock(ctx context.Context, resource string, token LockToken) error

	// SetUploadProgress stores the current upload progress percentage for a session.
	SetUploadProgress(ctx context.Context, uploadID string, percent int, ttl time.Duration) error

	// GetUploadProgress retrieves the current upload progress for a session.
	GetUploadProgress(ctx context.Context, uploadID string) (percent int, found bool, err error)

	// SetProcessingProgress stores the current processing progress for a media item.
	SetProcessingProgress(ctx context.Context, mediaID string, percent int, ttl time.Duration) error

	// GetProcessingProgress retrieves the current processing progress for a media item.
	GetProcessingProgress(ctx context.Context, mediaID string) (percent int, found bool, err error)

	// DeleteKey removes a single Redis key (e.g., on cleanup).
	DeleteKey(ctx context.Context, key string) error
}
