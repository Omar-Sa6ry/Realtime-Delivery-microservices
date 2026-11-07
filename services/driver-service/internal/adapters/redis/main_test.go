package redis_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	redisAddr   = "127.0.0.1:6380"
	geoKey      = "drivers:locations"
	redisPrefix = "driver-svc-test:"
)

const redisSkipMsg = "Redis is not reachable at " + redisAddr + "; start the e2e stack with " +
	"`docker compose -f docker-compose.e2e.yml -p driver-e2e up -d --wait` from the driver-service directory"

var (
	testRedisClient *goredis.Client
	redisAvailable  bool
)

func TestMain(m *testing.M) {
	client := goredis.NewClient(&goredis.Options{
		Addr:        redisAddr,
		DialTimeout: 2 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	err := client.Ping(ctx).Err()
	cancel()

	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %v\nskipping all redis adapter tests.\n", err)
		_ = client.Close()
		redisAvailable = false
	} else {
		redisAvailable = true
		testRedisClient = client
	}

	code := m.Run()

	if redisAvailable {
		// Best-effort cleanup of everything this suite may have created.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		for _, pattern := range []string{redisPrefix + "*", "idempotency:" + redisPrefix + "*", "lock:driver:*"} {
			keys, _ := testRedisClient.Keys(ctx, pattern).Result()
			if len(keys) > 0 {
				_ = testRedisClient.Del(ctx, keys...).Err()
			}
		}
		_ = testRedisClient.Del(ctx, geoKey).Err()
		cancel()
		_ = testRedisClient.Close()
	}
	os.Exit(code)
}

// requireRedis skips the test when the e2e Redis is unreachable.
func requireRedis(t *testing.T) *goredis.Client {
	t.Helper()
	if !redisAvailable {
		t.Skip(redisSkipMsg)
	}
	return testRedisClient
}

// testKey returns a unique key for a test.
func testKey(t *testing.T, name string) string {
	t.Helper()
	return redisPrefix + name
}

// cancelledCtx returns an already-cancelled context for exercising error paths.
func cancelledCtx() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx, cancel
}

// closedClient returns a redis client whose connection has been closed, used
// to exercise the error paths of the adapters without touching the shared
// client.
func closedClient(t *testing.T) *goredis.Client {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{Addr: redisAddr, DialTimeout: time.Second})
	if err := client.Close(); err != nil {
		t.Fatalf("failed to close probe client: %v", err)
	}
	return client
}
