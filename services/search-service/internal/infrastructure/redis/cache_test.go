package redis

import (
	"context"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/config"
	miniredis "github.com/alicebob/miniredis/v2"
)

func newTestCache(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	cfg := &config.Config{
		RedisHost:     mr.Host(),
		RedisPort:     mr.Port(),
		RedisTTL:      30 * time.Second,
		RedisDB:       0,
		RedisPassword: "",
	}
	return NewCache(cfg), mr
}

func TestCache_SetGet_RoundTrip(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()

	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	in := payload{Name: "x", Count: 3}
	key := c.GenerateKey("test", in)

	if err := c.Set(ctx, key, in); err != nil {
		t.Fatalf("Set: %v", err)
	}
	var out payload
	if !c.Get(ctx, key, &out) {
		t.Fatal("expected cache hit")
	}
	if out != in {
		t.Errorf("got %+v, want %+v", out, in)
	}
}

func TestCache_Get_Miss(t *testing.T) {
	c, _ := newTestCache(t)
	var dest map[string]interface{}
	if c.Get(context.Background(), "missing-key", &dest) {
		t.Fatal("expected miss")
	}
}

func TestCache_Get_CorruptValue(t *testing.T) {
	c, mr := newTestCache(t)
	mr.Set("bad", "{not json")
	var dest map[string]interface{}
	if c.Get(context.Background(), "bad", &dest) {
		t.Fatal("corrupt value should return false")
	}
}

func TestCache_Set_ExplicitTTL(t *testing.T) {
	c, mr := newTestCache(t)
	ctx := context.Background()
	if err := c.Set(ctx, "k", "v", 5*time.Second); err != nil {
		t.Fatalf("Set: %v", err)
	}
	ttl := mr.TTL("k")
	if ttl != 5*time.Second {
		t.Errorf("ttl = %v, want 5s", ttl)
	}
}

func TestCache_Set_DefaultTTLFromConfig(t *testing.T) {
	c, mr := newTestCache(t)
	ctx := context.Background()
	// no explicit TTL → config TTL (30s)
	if err := c.Set(ctx, "k2", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	ttl := mr.TTL("k2")
	if ttl != 30*time.Second {
		t.Errorf("ttl = %v, want 30s", ttl)
	}
	// explicit zero/negative TTL also falls back to config TTL
	if err := c.Set(ctx, "k3", "v", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if ttl := mr.TTL("k3"); ttl != 30*time.Second {
		t.Errorf("ttl = %v, want 30s", ttl)
	}
}

func TestCache_Set_MarshalError(t *testing.T) {
	c, _ := newTestCache(t)
	err := c.Set(context.Background(), "k", make(chan int))
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestCache_GenerateKey_DeterministicAndPrefixed(t *testing.T) {
	c, _ := newTestCache(t)
	k1 := c.GenerateKey("deliveries", map[string]string{"a": "1"})
	k2 := c.GenerateKey("deliveries", map[string]string{"a": "1"})
	k3 := c.GenerateKey("drivers", map[string]string{"a": "1"})
	k4 := c.GenerateKey("deliveries", map[string]string{"a": "2"})

	if k1 != k2 {
		t.Error("same input should produce same key")
	}
	if k1 == k3 {
		t.Error("different prefixes should produce different keys")
	}
	if k1 == k4 {
		t.Error("different inputs should produce different keys")
	}
	if len(k1) < len("search:deliveries:") || k1[:len("search:deliveries:")] != "search:deliveries:" {
		t.Errorf("key missing prefix: %s", k1)
	}
}

func TestCache_GenerateKey_UnmarshalableInputIgnored(t *testing.T) {
	c, _ := newTestCache(t)
	k := c.GenerateKey("p", make(chan int))
	if k == "" {
		t.Fatal("marshal failure should still produce a key")
	}
}

func TestCache_Ping(t *testing.T) {
	c, _ := newTestCache(t)
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestCache_Ping_ErrorWhenDown(t *testing.T) {
	c, mr := newTestCache(t)
	mr.Close()
	if err := c.Ping(context.Background()); err == nil {
		t.Fatal("expected ping error when server is down")
	}
}
