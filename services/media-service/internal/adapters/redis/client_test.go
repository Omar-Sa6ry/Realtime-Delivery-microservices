package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestNewClientConnectsWhenRedisIsReady(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(s.Close)

	client, err := NewClient(s.Addr(), "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
