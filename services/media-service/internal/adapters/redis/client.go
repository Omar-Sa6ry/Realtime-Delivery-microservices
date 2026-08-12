package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

// NewClient creates and validates a Redis client connection.
// It is a thin wrapper around go-redis that pings on startup.
func NewClient(addr, password string) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
		// Connection pool settings for a high-throughput service.
		PoolSize:     50,
		MinIdleConns: 10,
	})

	ctx := context.Background()
	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("redis ping failed at %q: %w", addr, err)
	}

	return client, nil
}
