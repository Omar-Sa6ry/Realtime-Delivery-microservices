package redis_test

import (
	"context"
	"testing"
	"time"

	redisadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/redis"
)

// lockKey returns the underlying redis key of a lock resource.
func lockKey(resource string) string {
	return "lock:driver:" + resource
}

func TestLockManagerAcquire(t *testing.T) {
	client := requireRedis(t)
	manager := redisadapter.NewLockManager(client)
	ctx := context.Background()

	t.Run("acquires a free lock and sets its TTL", func(t *testing.T) {
		resource := testKey(t, "lock-acquire")
		if err := client.Del(ctx, lockKey(resource)).Err(); err != nil {
			t.Fatalf("cleanup lock: %v", err)
		}

		ok, err := manager.Acquire(ctx, resource, 5*time.Second)
		if err != nil {
			t.Fatalf("Acquire() error: %v", err)
		}
		if !ok {
			t.Fatal("Acquire() = false for a free lock, want true")
		}

		ttl, err := client.PTTL(ctx, lockKey(resource)).Result()
		if err != nil {
			t.Fatalf("PTTL: %v", err)
		}
		if ttl <= 0 || ttl > 5*time.Second {
			t.Errorf("lock TTL = %v, want (0, 5s]", ttl)
		}
	})

	t.Run("denies a second acquisition (no re-entrancy)", func(t *testing.T) {
		resource := testKey(t, "lock-reentrant")
		if err := client.Del(ctx, lockKey(resource)).Err(); err != nil {
			t.Fatalf("cleanup lock: %v", err)
		}
		if ok, err := manager.Acquire(ctx, resource, time.Minute); err != nil || !ok {
			t.Fatalf("first Acquire() = (%v, %v), want (true, nil)", ok, err)
		}
		ok, err := manager.Acquire(ctx, resource, time.Minute)
		if err != nil {
			t.Fatalf("second Acquire() error: %v", err)
		}
		if ok {
			t.Error("second Acquire() = true, want false while the lock is held")
		}
	})

	t.Run("allows acquisition after the lock expires", func(t *testing.T) {
		resource := testKey(t, "lock-expiry")
		if err := client.Del(ctx, lockKey(resource)).Err(); err != nil {
			t.Fatalf("cleanup lock: %v", err)
		}
		if ok, err := manager.Acquire(ctx, resource, 100*time.Millisecond); err != nil || !ok {
			t.Fatalf("first Acquire() = (%v, %v), want (true, nil)", ok, err)
		}
		time.Sleep(300 * time.Millisecond)
		ok, err := manager.Acquire(ctx, resource, time.Minute)
		if err != nil {
			t.Fatalf("Acquire() after expiry error: %v", err)
		}
		if !ok {
			t.Error("Acquire() after expiry = false, want true")
		}
	})

	t.Run("propagates a closed client error", func(t *testing.T) {
		closed := redisadapter.NewLockManager(closedClient(t))
		ok, err := closed.Acquire(ctx, "any", time.Second)
		if err == nil {
			t.Fatal("Acquire() expected an error for a closed client, got nil")
		}
		if ok {
			t.Error("Acquire() = true on error, want false")
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		ok, err := manager.Acquire(cctx, testKey(t, "lock-cancelled"), time.Second)
		if err == nil {
			t.Fatal("Acquire() expected an error for a cancelled context, got nil")
		}
		if ok {
			t.Error("Acquire() = true on error, want false")
		}
	})
}

func TestLockManagerRelease(t *testing.T) {
	client := requireRedis(t)
	manager := redisadapter.NewLockManager(client)
	ctx := context.Background()

	t.Run("releases a held lock so it can be acquired again", func(t *testing.T) {
		resource := testKey(t, "lock-release")
		if err := client.Del(ctx, lockKey(resource)).Err(); err != nil {
			t.Fatalf("cleanup lock: %v", err)
		}
		if ok, err := manager.Acquire(ctx, resource, time.Minute); err != nil || !ok {
			t.Fatalf("Acquire() = (%v, %v), want (true, nil)", ok, err)
		}
		// The token argument is currently ignored by the implementation: any
		// token releases the lock.
		if err := manager.Release(ctx, resource, "some-token"); err != nil {
			t.Fatalf("Release() error: %v", err)
		}
		exists, err := client.Exists(ctx, lockKey(resource)).Result()
		if err != nil {
			t.Fatalf("EXISTS: %v", err)
		}
		if exists != 0 {
			t.Errorf("lock still exists after Release() (EXISTS = %d)", exists)
		}
		if ok, err := manager.Acquire(ctx, resource, time.Minute); err != nil || !ok {
			t.Errorf("Acquire() after Release() = (%v, %v), want (true, nil)", ok, err)
		}
	})

	t.Run("does not fail when the lock is not held", func(t *testing.T) {
		if err := manager.Release(ctx, testKey(t, "lock-never-held"), "any-token"); err != nil {
			t.Errorf("Release() error for an unheld lock: %v", err)
		}
	})

	t.Run("propagates a closed client error", func(t *testing.T) {
		closed := redisadapter.NewLockManager(closedClient(t))
		if err := closed.Release(ctx, "any", "any-token"); err == nil {
			t.Error("Release() expected an error for a closed client, got nil")
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		if err := manager.Release(cctx, testKey(t, "lock-release-cancelled"), "any-token"); err == nil {
			t.Error("Release() expected an error for a cancelled context, got nil")
		}
	})
}
