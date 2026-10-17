package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type IdempotencyCache struct {
	client *redis.Client
	ctx    context.Context
}

func NewIdempotencyCache(client *redis.Client, ctx context.Context) *IdempotencyCache {
	return &IdempotencyCache{client: client, ctx: ctx}
}

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

func (c *IdempotencyCache) Save(key string, value string, expiry time.Duration) error {
	err := c.client.Set(c.ctx, key, value, expiry).Err()
	if err != nil {
		return fmt.Errorf("failed to save idempotency cache: %w", err)
	}
	return nil
}

func (c *IdempotencyCache) Delete(key string) error {
	err := c.client.Del(c.ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete idempotency cache key: %w", err)
	}
	return nil
}
