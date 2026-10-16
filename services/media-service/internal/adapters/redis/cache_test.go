package redis

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func newTestCache(t *testing.T) (*CacheAdapter, *goredis.Client, *miniredis.Miniredis) {
	t.Helper()
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(s.Close)

	client := goredis.NewClient(&goredis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	return NewCacheAdapter(client), client, s
}

func newClosedCache(t *testing.T) *CacheAdapter {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	return NewCacheAdapter(client)
}

func TestCheckAndStoreIdempotencyStoresThenReplays(t *testing.T) {
	c, _, _ := newTestCache(t)
	ctx := context.Background()
	response := []byte(`{"mediaId":"m1"}`)

	first, err := c.CheckAndStoreIdempotency(ctx, "user-1", "req-1", response, time.Minute)
	if err != nil {
		t.Fatalf("first CheckAndStoreIdempotency: %v", err)
	}
	if first.Exists {
		t.Fatal("first call must report Exists=false")
	}

	second, err := c.CheckAndStoreIdempotency(ctx, "user-1", "req-1", response, time.Minute)
	if err != nil {
		t.Fatalf("second CheckAndStoreIdempotency: %v", err)
	}
	if !second.Exists {
		t.Fatal("duplicate call must report Exists=true")
	}
	if string(second.CachedResponse) != string(response) {
		t.Fatalf("cached response = %q, want %q", second.CachedResponse, response)
	}

	other, err := c.CheckAndStoreIdempotency(ctx, "user-1", "req-2", response, time.Minute)
	if err != nil {
		t.Fatalf("different key CheckAndStoreIdempotency: %v", err)
	}
	if other.Exists {
		t.Fatal("a different key must not report Exists=true")
	}
}

func TestCheckAndStoreIdempotencyRejectsCorruptCachedValue(t *testing.T) {
	c, client, _ := newTestCache(t)
	ctx := context.Background()

	if err := client.Set(ctx, "idempotency:user-1:req-1", "not-json", 0).Err(); err != nil {
		t.Fatalf("seed corrupt value: %v", err)
	}

	_, err := c.CheckAndStoreIdempotency(ctx, "user-1", "req-1", []byte("x"), time.Minute)
	if err == nil || !strings.Contains(err.Error(), "unmarshal idempotency response") {
		t.Fatalf("expected unmarshal error, got %v", err)
	}
}

func TestCheckAndStoreIdempotencyGetWrongTypeError(t *testing.T) {
	c, client, _ := newTestCache(t)
	ctx := context.Background()

	if err := client.LPush(ctx, "idempotency:user-1:req-1", "element").Err(); err != nil {
		t.Fatalf("seed list value: %v", err)
	}

	_, err := c.CheckAndStoreIdempotency(ctx, "user-1", "req-1", []byte("x"), time.Minute)
	if err == nil || !strings.Contains(err.Error(), "redis Get idempotency cached response") {
		t.Fatalf("expected non-Nil Get error, got %v", err)
	}
}

func TestCheckAndStoreIdempotencyClosedClient(t *testing.T) {
	c := newClosedCache(t)

	_, err := c.CheckAndStoreIdempotency(context.Background(), "user-1", "req-1", nil, time.Minute)
	if err == nil || !strings.Contains(err.Error(), "redis SetNX idempotency key") {
		t.Fatalf("expected SetNX error, got %v", err)
	}
}

func TestCheckAndStoreIdempotencyKeyExpiry(t *testing.T) {
	c, _, s := newTestCache(t)
	ctx := context.Background()

	if _, err := c.CheckAndStoreIdempotency(ctx, "user-1", "req-1", []byte("a"), 10*time.Second); err != nil {
		t.Fatalf("seed idempotency key: %v", err)
	}

	s.FastForward(11 * time.Second)

	res, err := c.CheckAndStoreIdempotency(ctx, "user-1", "req-1", []byte("a"), 10*time.Second)
	if err != nil {
		t.Fatalf("after expiry CheckAndStoreIdempotency: %v", err)
	}
	if res.Exists {
		t.Fatal("expired key must be reported as a new request")
	}
}

func TestAcquireLockLifecycle(t *testing.T) {
	c, client, _ := newTestCache(t)
	ctx := context.Background()

	token, ok, err := c.AcquireLock(ctx, "m1", time.Minute)
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	if !ok {
		t.Fatal("first AcquireLock should be granted")
	}
	if token == "" {
		t.Fatal("granted lock must carry a non-empty token")
	}

	_, ok, err = c.AcquireLock(ctx, "m1", time.Minute)
	if err != nil {
		t.Fatalf("second AcquireLock: %v", err)
	}
	if ok {
		t.Fatal("second AcquireLock on the same resource must be denied")
	}

	_, ok, err = c.AcquireLock(ctx, "m2", time.Minute)
	if err != nil {
		t.Fatalf("AcquireLock other resource: %v", err)
	}
	if !ok {
		t.Fatal("a different resource must be lockable")
	}

	exists, err := client.Exists(ctx, "lock:media:m1").Result()
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists != 1 {
		t.Fatalf("lock key exists = %d, want 1", exists)
	}
}

func TestAcquireLockClosedClient(t *testing.T) {
	c := newClosedCache(t)

	_, _, err := c.AcquireLock(context.Background(), "m1", time.Minute)
	if err == nil || !strings.Contains(err.Error(), "redis AcquireLock SetNX") {
		t.Fatalf("expected SetNX error, got %v", err)
	}
}

func TestReleaseLockWithMatchingToken(t *testing.T) {
	c, client, _ := newTestCache(t)
	ctx := context.Background()

	token, ok, err := c.AcquireLock(ctx, "m1", time.Minute)
	if err != nil || !ok {
		t.Fatalf("AcquireLock: ok=%v err=%v", ok, err)
	}

	if err := c.ReleaseLock(ctx, "m1", token); err != nil {
		t.Fatalf("ReleaseLock: %v", err)
	}

	exists, err := client.Exists(ctx, "lock:media:m1").Result()
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists != 0 {
		t.Fatalf("lock key exists = %d, want 0 after release", exists)
	}
}

func TestReleaseLockWithMismatchedTokenKeepsLock(t *testing.T) {
	c, client, _ := newTestCache(t)
	ctx := context.Background()

	if _, ok, err := c.AcquireLock(ctx, "m1", time.Minute); err != nil || !ok {
		t.Fatalf("AcquireLock: ok=%v err=%v", ok, err)
	}

	if err := c.ReleaseLock(ctx, "m1", "wrong-token"); err != nil {
		t.Fatalf("ReleaseLock with wrong token: %v", err)
	}

	exists, err := client.Exists(ctx, "lock:media:m1").Result()
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists != 1 {
		t.Fatalf("lock key exists = %d, want 1 after mismatched release", exists)
	}
}

func TestReleaseLockClosedClient(t *testing.T) {
	c := newClosedCache(t)

	err := c.ReleaseLock(context.Background(), "m1", "token")
	if err == nil || !strings.Contains(err.Error(), "redis ReleaseLock script") {
		t.Fatalf("expected script error, got %v", err)
	}
}

func TestUploadProgressRoundTrip(t *testing.T) {
	c, client, _ := newTestCache(t)
	ctx := context.Background()

	if err := c.SetUploadProgress(ctx, "u1", 42, time.Minute); err != nil {
		t.Fatalf("SetUploadProgress: %v", err)
	}

	percent, ok, err := c.GetUploadProgress(ctx, "u1")
	if err != nil {
		t.Fatalf("GetUploadProgress: %v", err)
	}
	if !ok || percent != 42 {
		t.Fatalf("GetUploadProgress = (%d, %v), want (42, true)", percent, ok)
	}

	if _, ok, err := c.GetUploadProgress(ctx, "missing"); err != nil || ok {
		t.Fatalf("missing upload progress = (ok=%v, err=%v), want (false, nil)", ok, err)
	}

	if err := client.Set(ctx, "media:upload:progress:u2", "not-a-number", 0).Err(); err != nil {
		t.Fatalf("seed non-integer progress: %v", err)
	}
	if _, _, err := c.GetUploadProgress(ctx, "u2"); err == nil || !strings.Contains(err.Error(), "redis GetUploadProgress") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestUploadProgressClosedClient(t *testing.T) {
	c := newClosedCache(t)

	if err := c.SetUploadProgress(context.Background(), "u1", 10, time.Minute); err == nil {
		t.Fatal("SetUploadProgress on closed client must fail")
	}
	if _, _, err := c.GetUploadProgress(context.Background(), "u1"); err == nil || !strings.Contains(err.Error(), "redis GetUploadProgress") {
		t.Fatalf("expected GetUploadProgress error, got %v", err)
	}
}

func TestUploadProgressExpires(t *testing.T) {
	c, _, s := newTestCache(t)
	ctx := context.Background()

	if err := c.SetUploadProgress(ctx, "u1", 50, 5*time.Second); err != nil {
		t.Fatalf("SetUploadProgress: %v", err)
	}
	s.FastForward(6 * time.Second)

	if _, ok, err := c.GetUploadProgress(ctx, "u1"); err != nil || ok {
		t.Fatalf("expired progress = (ok=%v, err=%v), want (false, nil)", ok, err)
	}
}

func TestProcessingProgressRoundTrip(t *testing.T) {
	c, client, _ := newTestCache(t)
	ctx := context.Background()

	if err := c.SetProcessingProgress(ctx, "m1", 80, time.Minute); err != nil {
		t.Fatalf("SetProcessingProgress: %v", err)
	}

	percent, ok, err := c.GetProcessingProgress(ctx, "m1")
	if err != nil {
		t.Fatalf("GetProcessingProgress: %v", err)
	}
	if !ok || percent != 80 {
		t.Fatalf("GetProcessingProgress = (%d, %v), want (80, true)", percent, ok)
	}

	if _, ok, err := c.GetProcessingProgress(ctx, "missing"); err != nil || ok {
		t.Fatalf("missing processing progress = (ok=%v, err=%v), want (false, nil)", ok, err)
	}

	if err := client.Set(ctx, "media:processing:progress:m2", "abc", 0).Err(); err != nil {
		t.Fatalf("seed non-integer progress: %v", err)
	}
	if _, _, err := c.GetProcessingProgress(ctx, "m2"); err == nil || !strings.Contains(err.Error(), "redis GetProcessingProgress") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestProcessingProgressClosedClient(t *testing.T) {
	c := newClosedCache(t)

	if err := c.SetProcessingProgress(context.Background(), "m1", 10, time.Minute); err == nil {
		t.Fatal("SetProcessingProgress on closed client must fail")
	}
	if _, _, err := c.GetProcessingProgress(context.Background(), "m1"); err == nil || !strings.Contains(err.Error(), "redis GetProcessingProgress") {
		t.Fatalf("expected GetProcessingProgress error, got %v", err)
	}
}

func TestDeleteKey(t *testing.T) {
	c, client, _ := newTestCache(t)
	ctx := context.Background()

	if err := client.Set(ctx, "k1", "v", 0).Err(); err != nil {
		t.Fatalf("seed key: %v", err)
	}

	if err := c.DeleteKey(ctx, "k1"); err != nil {
		t.Fatalf("DeleteKey: %v", err)
	}

	exists, err := client.Exists(ctx, "k1").Result()
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists != 0 {
		t.Fatalf("key exists = %d, want 0 after delete", exists)
	}
}

func TestDeleteKeyClosedClient(t *testing.T) {
	c := newClosedCache(t)

	if err := c.DeleteKey(context.Background(), "k1"); err == nil {
		t.Fatal("DeleteKey on closed client must fail")
	}
}
