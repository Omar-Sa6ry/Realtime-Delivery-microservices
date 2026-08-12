package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
	goredis "github.com/redis/go-redis/v9"
)

const (
	idempotencyKeyPrefix = "idempotency"
	lockKeyPrefix        = "lock:media"
	uploadProgressPrefix = "media:upload:progress"
	processingPrefix     = "media:processing:progress"
)

// CacheAdapter implements ports.Cache using Redis.
type CacheAdapter struct {
	client *goredis.Client
}

// NewCacheAdapter creates a new CacheAdapter backed by the given Redis client.
func NewCacheAdapter(client *goredis.Client) *CacheAdapter {
	return &CacheAdapter{client: client}
}

// CheckAndStoreIdempotency atomically checks and stores an idempotency key.
// Uses SET NX (set-if-not-exists) for atomicity — no WATCH/MULTI needed.
func (c *CacheAdapter) CheckAndStoreIdempotency(ctx context.Context, userID, key string, response []byte, ttl time.Duration) (*ports.IdempotencyResult, error) {
	redisKey := fmt.Sprintf("%s:%s:%s", idempotencyKeyPrefix, userID, key)

	type stored struct {
		Response []byte `json:"response"`
	}

	payload, err := json.Marshal(stored{Response: response})
	if err != nil {
		return nil, fmt.Errorf("marshal idempotency payload: %w", err)
	}

	// SET NX returns OK (true) if the key was newly set, nil (false) if it already existed.
	ok, err := c.client.SetNX(ctx, redisKey, payload, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("redis SetNX idempotency key: %w", err)
	}

	if ok {
		// Key was freshly created — this is a new request.
		return &ports.IdempotencyResult{Exists: false}, nil
	}

	// Key already existed — this is a duplicate request; retrieve the cached response.
	raw, err := c.client.Get(ctx, redisKey).Bytes()
	if err != nil {
		// Key might have expired between SetNX check and Get — treat as new request.
		if err == goredis.Nil {
			return &ports.IdempotencyResult{Exists: false}, nil
		}
		return nil, fmt.Errorf("redis Get idempotency cached response: %w", err)
	}

	var s stored
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("unmarshal idempotency response: %w", err)
	}

	return &ports.IdempotencyResult{Exists: true, CachedResponse: s.Response}, nil
}

// AcquireLock uses SET NX PX to acquire a named distributed lock.
// The token is a unique value that prevents a different caller from releasing the lock.
func (c *CacheAdapter) AcquireLock(ctx context.Context, resource string, ttl time.Duration) (ports.LockToken, bool, error) {
	key := fmt.Sprintf("%s:%s", lockKeyPrefix, resource)
	token := ports.LockToken(fmt.Sprintf("%d", time.Now().UnixNano()))

	ok, err := c.client.SetNX(ctx, key, string(token), ttl).Result()
	if err != nil {
		return "", false, fmt.Errorf("redis AcquireLock SetNX: %w", err)
	}
	return token, ok, nil
}

// ReleaseLock uses a Lua script to atomically check the token and delete the key.
// This prevents a lock holder from releasing another holder's lock.
var releaseLockScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`)

// ReleaseLock releases a distributed lock. It is a no-op if the token does not match.
func (c *CacheAdapter) ReleaseLock(ctx context.Context, resource string, token ports.LockToken) error {
	key := fmt.Sprintf("%s:%s", lockKeyPrefix, resource)
	if err := releaseLockScript.Run(ctx, c.client, []string{key}, string(token)).Err(); err != nil && err != goredis.Nil {
		return fmt.Errorf("redis ReleaseLock script: %w", err)
	}
	return nil
}

// SetUploadProgress stores upload progress percentage for a session with TTL.
// Uses Redis TTL instead of a cron job for automatic cleanup.
func (c *CacheAdapter) SetUploadProgress(ctx context.Context, uploadID string, percent int, ttl time.Duration) error {
	key := fmt.Sprintf("%s:%s", uploadProgressPrefix, uploadID)
	return c.client.Set(ctx, key, percent, ttl).Err()
}

// GetUploadProgress retrieves the cached upload progress for a session.
func (c *CacheAdapter) GetUploadProgress(ctx context.Context, uploadID string) (int, bool, error) {
	key := fmt.Sprintf("%s:%s", uploadProgressPrefix, uploadID)
	val, err := c.client.Get(ctx, key).Int()
	if err == goredis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("redis GetUploadProgress: %w", err)
	}
	return val, true, nil
}

// SetProcessingProgress stores processing progress percentage for a media item.
func (c *CacheAdapter) SetProcessingProgress(ctx context.Context, mediaID string, percent int, ttl time.Duration) error {
	key := fmt.Sprintf("%s:%s", processingPrefix, mediaID)
	return c.client.Set(ctx, key, percent, ttl).Err()
}

// GetProcessingProgress retrieves the cached processing progress for a media item.
func (c *CacheAdapter) GetProcessingProgress(ctx context.Context, mediaID string) (int, bool, error) {
	key := fmt.Sprintf("%s:%s", processingPrefix, mediaID)
	val, err := c.client.Get(ctx, key).Int()
	if err == goredis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("redis GetProcessingProgress: %w", err)
	}
	return val, true, nil
}

// DeleteKey removes a single Redis key.
func (c *CacheAdapter) DeleteKey(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}
