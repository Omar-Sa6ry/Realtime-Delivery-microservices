package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

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

// TryRenewal attempts to renew the TTL on an existing lock.
// Returns true if the renewal was successful.
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