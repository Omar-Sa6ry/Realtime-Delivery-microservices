package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheAdapter struct {
	client *redis.Client
}

func NewCacheAdapter(client *redis.Client) *CacheAdapter {
	return &CacheAdapter{client: client}
}

func (c *CacheAdapter) CheckAndStoreIdempotency(ctx context.Context, key string, result []byte, ttl time.Duration) (bool, error) {
	// Use SET NX for atomic check-and-store
	resultStr := string(result)
	ok, err := c.client.SetNX(ctx, "idempotency:"+key, resultStr, ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (c *CacheAdapter) Exists(ctx context.Context, key string) (bool, error) {
	_, err := c.client.Exists(ctx, "idempotency:"+key).Result()
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *CacheAdapter) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, "idempotency:"+key).Err()
}