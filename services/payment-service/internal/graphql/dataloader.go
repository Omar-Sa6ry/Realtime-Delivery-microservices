package graphql

import (
	"context"
)

// PaymentLoader loads Payment entities by ID using DataLoader to prevent N+1 query issues.
type PaymentLoader struct {
	base func(ctx context.Context, ids []any) ([]interface{}, error)
}

// NewPaymentLoader creates a new PaymentLoader.
func NewPaymentLoader(loadFn func(ctx context.Context, ids []any) ([]interface{}, error)) *PaymentLoader {
	return &PaymentLoader{base: loadFn}
}

// Load loads a Payment by ID.
func (l *PaymentLoader) Load(ctx context.Context, id any) (interface{}, error) {
	results, err := l.base(ctx, []any{id})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

// InitializePaymentLoader initializes the PaymentDataLoader with a batch function.
func InitializePaymentLoader(loadFn func(ctx context.Context, ids []any) ([]interface{}, error)) *PaymentLoader {
	return &PaymentLoader{base: loadFn}
}

// LoadPayment loads a single Payment by ID using the DataLoader.
func LoadPayment(ctx context.Context, loader *PaymentLoader, id any) (interface{}, error) {
	return loader.Load(ctx, id)
}

// LoadPaymentBatch loads multiple Payments by IDs using the DataLoader.
func LoadPaymentBatch(ctx context.Context, loader *PaymentLoader, ids []any) ([]interface{}, error) {
	return loader.base(ctx, ids)
}