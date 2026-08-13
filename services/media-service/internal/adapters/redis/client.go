package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// NewClient creates and validates a Redis client connection with exponential back-off retry.
// It uses DB=0 (the single shared database) and retries up to 30 times with 5-second
// intervals, making the service resilient to slow Redis startup in Kubernetes.
func NewClient(addr, password string) (*goredis.Client, error) {
	const (
		maxAttempts = 30
		retryDelay  = 5 * time.Second
	)

	var client *goredis.Client
	var lastErr error

	for i := 1; i <= maxAttempts; i++ {
		client = goredis.NewClient(&goredis.Options{
			Addr:     addr,
			Password: password,
			DB:       0, // Shared DB=0 across all services — do NOT change without updating all consumers.

			// Connection pool settings for a high-throughput service.
			PoolSize:        50,
			MinIdleConns:    10,
			ConnMaxIdleTime: 5 * time.Minute,
			DialTimeout:     5 * time.Second,
			ReadTimeout:     3 * time.Second,
			WriteTimeout:    3 * time.Second,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, lastErr = client.Ping(ctx).Result()
		cancel()

		if lastErr == nil {
			slog.Info("Redis ready", "addr", addr, "db", 0, "attempt", i)
			return client, nil
		}

		slog.Warn("Redis not ready, retrying...",
			"addr", addr,
			"attempt", i,
			"maxAttempts", maxAttempts,
			"error", lastErr,
			"retryIn", retryDelay,
		)

		// Close failed client before creating a new one to avoid connection leaks.
		_ = client.Close()

		if i < maxAttempts {
			time.Sleep(retryDelay)
		}
	}

	return nil, fmt.Errorf("redis ping failed at %q after %d attempts: %w", addr, maxAttempts, lastErr)
}
