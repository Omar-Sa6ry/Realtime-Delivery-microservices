package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type LockManager struct {
	client *redis.Client
}

func NewLockManager(client *redis.Client) *LockManager {
	return &LockManager{client: client}
}

func (l *LockManager) Acquire(ctx context.Context, resource string, ttl time.Duration) (bool, error) {
	// Use SET NX for atomic lock acquisition
	result, err := l.client.SetNX(ctx, "lock:driver:"+resource, "1", ttl).Result()
	if err != nil {
		return false, err
	}
	return result, nil
}

func (l *LockManager) Release(ctx context.Context, resource string, token string) error {
	// Release the lock by deleting the key
	return l.client.Del(ctx, "lock:driver:"+resource).Err()
}