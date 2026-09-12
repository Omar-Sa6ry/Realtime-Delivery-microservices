package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/realtime-delivery/payment-service/internal/domain"
)

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
	result, err := c.client.Get(c.ctx, key).Result()
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

// LockManager provides distributed locking with TTL for concurrent processing safety.
type LockManager struct {
	client   *redis.Client
	ctx      context.Context
	lockName string
	ttl      time.Duration
}

// NewLockManager creates a new LockManager.
func NewLockManager(client *redis.Client, ctx context.Context, lockName string, ttl time.Duration) *LockManager {
	return &LockManager{client: client, ctx: ctx, lockName: lockName, ttl: ttl}
}

func (l *LockManager) Acquire() (bool, error) {
	// Use SET NX XX for atomic lock acquire with renewal
	result, err := l.client.Set(l.ctx, l.lockName, "1", l.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}
	// If the key was set (NX means it didn't exist), we acquired the lock
	if result == "OK" {
		return true, nil
	}
	// Key already exists, lock is held by another process
	return false, nil
}

func (l *LockManager) TryRenewal() (bool, error) {
	// Use Lua script for atomic check-and-renew
	script := `
	if redis.call("exists", KEYS[1]) then
		return redis.call("expire", KEYS[1], ARGV[1])
	end
	return 0
`
	result, err := l.client.Eval(l.ctx, script, []string{l.lockName}, l.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to renew lock: %w", err)
	}
	if result == 1 {
		return true, nil
	}
	return false, nil
}

// Release releases a distributed lock.
func (l *LockManager) Release() error {
	err := l.client.Del(l.ctx, l.lockName).Err()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	return nil
}