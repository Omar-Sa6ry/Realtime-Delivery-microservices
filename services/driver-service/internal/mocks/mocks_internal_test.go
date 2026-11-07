package mocks

import (
	"context"
	"errors"
	"testing"
)

// MockIdempotencyStore has no exported error setter (its errors map is
// unexported and mocks.go must not be modified), so this in-package test
// injects errors directly to exercise the error branches of
// CheckAndStore, Exists and Delete.
func TestMockIdempotencyStoreErrorBranches(t *testing.T) {
	ctx := context.Background()
	s := NewMockIdempotencyStore()
	injected := errors.New("idempotency failure")
	s.errors["bad-key"] = injected

	if _, err := s.CheckAndStore(ctx, "bad-key", []byte("x"), 1); !errors.Is(err, injected) {
		t.Errorf("CheckAndStore() err = %v, want %v", err, injected)
	}
	if _, err := s.Exists(ctx, "bad-key"); !errors.Is(err, injected) {
		t.Errorf("Exists() err = %v, want %v", err, injected)
	}
	if err := s.Delete(ctx, "bad-key"); !errors.Is(err, injected) {
		t.Errorf("Delete() err = %v, want %v", err, injected)
	}
}
