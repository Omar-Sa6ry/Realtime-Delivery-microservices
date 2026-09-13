package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/realtime-delivery/payment-service/internal/domain"
)

// IdempotencyCache provides fast-path idempotency key checking using Redis.
type IdempotencyCache struct {
	client *redis.Client
	ctx    context.Context
}

// NewIdempotencyCache creates a new IdempotencyCache.
func NewIdempotencyCache(client *redis.Client, ctx context.Context) *IdempotencyCache {
	return &IdempotencyCache{client: client, ctx: ctx}
}

// CheckExists checks if an idempotency key already exists.
func (c *IdempotencyCache) CheckExists(key string) (bool, error) {
	_, err := c.client.Get(c.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, fmt.Errorf("failed to check idempotency cache: %w", err)
	}
	return true, nil
}

// Save stores an idempotency key with its result and expiry.
func (c *IdempotencyCache) Save(key string, value string, expiry time.Duration) error {
	err := c.client.Set(c.ctx, key, value, expiry).Err()
	if err != nil {
		return fmt.Errorf("failed to save idempotency cache: %w", err)
	}
	return nil
}

// Delete removes an idempotency key from the cache.
func (c *IdempotencyCache) Delete(key string) error {
	err := c.client.Del(c.ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete idempotency cache key: %w", err)
	}
	return nil
}