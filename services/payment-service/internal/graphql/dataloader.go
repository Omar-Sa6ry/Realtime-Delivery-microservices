package graphql

import (
	"context"
	"fmt"
	"strings"

	"github.com/graph-gophers/dataloader/v7"
)

// PaymentLoader loads Payment entities by ID using DataLoader to prevent N+1 query issues.
type PaymentLoader struct {
	base dataloader.BatchFunc[any, *Payment]
}

// NewPaymentLoader creates a new PaymentLoader.
func NewPaymentLoader(loadFn dataloader.BatchFunc[any, *Payment]) *PaymentLoader {
	return &PaymentLoader{base: loadFn}
}

// Load loads a Payment by ID.
func (l *PaymentLoader) Load(ctx context.Context, id any) (*Payment, error) {
	return l.base(ctx, id)
}

// PaymentDataLoader provides a typed interface for loading Payment entities.
type PaymentDataLoader interface {
	Load(ctx context.Context, id any) (*Payment, error)
}

// PaymentBatchLoader loads multiple Payments by IDs.
type PaymentBatchLoader interface {
	LoadBatch(ctx context.Context, ids []any) (*[]*Payment, error)
}

// InitializePaymentLoader initializes the PaymentDataLoader with a batch function.
// This is typically called from the resolver setup.
func InitializePaymentLoader(loadFn dataloader.BatchFunc[any, *Payment]) *PaymentLoader {
	return &PaymentLoader{base: loadFn}
}

// LoadPayment loads a single Payment by ID using the DataLoader.
func LoadPayment(ctx context.Context, loader *PaymentLoader, id any) (*Payment, error) {
	return loader.Load(ctx, id)
}

// LoadPaymentBatch loads multiple Payments by IDs using the DataLoader.
func LoadPaymentBatch(ctx context.Context, loader *PaymentLoader, ids []any) (*[]*Payment, error) {
	return loader.base(ctx, ids)
}

// paymentFields defines the fields that can be loaded by DataLoader.
type paymentFields struct {
	id        string
	correlationID string
	causationID string
}

// newPaymentFields creates payment fields for DataLoader indexing.
func newPaymentFields(id string, correlationID string, causationID string) paymentFields {
	return paymentFields{
		id:        correlationID,
		correlationID: correlationID,
		causationID: causationID,
	}
}

// parsePaymentID parses the ID string into its components.
func parsePaymentID(id string) (string, string, string) {
	parts := strings.Split(id, ":")
	if len(parts) >= 3 {
		return parts[0], parts[1], parts[2]
	}
	return id, "", ""
}

// GetPaymentFromContext extracts the Payment from the GraphQL context.
func GetPaymentFromContext(ctx context.Context) (*Payment, error) {
	loader := dataloader.For[any, *Payment](ctx)
	if loader == nil {
		return nil, fmt.Errorf("payment loader not found in context")
	}
	return loader.Load(ctx, "")
}
`