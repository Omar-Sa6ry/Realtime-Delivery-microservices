package ports

import (
	"context"
)

type IdempotencyStore interface {
	CheckOrCreate(ctx context.Context, key, operation, requestHash string) (*IdempotencyRecord, bool, error)
	Complete(ctx context.Context, key string, responsePayload []byte) error
}

type IdempotencyRecord struct {
	Key           string
	Operation     string
	RequestHash   string
	ResponsePayload []byte
	Status        string // "pending", "completed", "failed"
	CreatedAt     int64
	CompletedAt   int64
}