package graphql

import (
	"context"
	"fmt"
	"time"
)

// Resolver is the GraphQL resolver root for payment operations.
type Resolver struct {
	paymentLoader *PaymentLoader
}

// NewResolver creates a new GraphQL resolver with the given data loaders.
func NewResolver(paymentLoader *PaymentLoader) *Resolver {
	return &Resolver{paymentLoader: paymentLoader}
}

// PaymentServiceInfo returns service health info (used by API Gateway health checks).
func (r *Resolver) PaymentServiceInfo(ctx context.Context) (map[string]interface{}, error) {
	now := timeNowUTC()
	return map[string]interface{}{
		"success":    true,
		"statusCode": 200,
		"message":    "Payment service is healthy",
		"timeStamp":  now,
		"data": map[string]interface{}{
			"name":    "payment-service",
			"version": "1.0.0",
			"status":  "healthy",
		},
	}, nil
}

// Payment returns a Payment entity by ID.
func (r *Resolver) Payment(ctx context.Context, id string) (*Payment, error) {
	if r.paymentLoader == nil {
		return nil, fmt.Errorf("payment loader not initialized")
	}
	return r.paymentLoader.Load(ctx, id)
}

// PaymentServiceInfoResolver returns the payment service info for the root query.
func (r *Resolver) PaymentServiceInfoResolver(ctx context.Context) (map[string]interface{}, error) {
	return r.PaymentServiceInfo(ctx)
}

// PaymentResolver returns a Payment entity for the GraphQL query.
func (r *Resolver) PaymentResolver(ctx context.Context, id string) (*Payment, error) {
	return r.Payment(ctx, id)
}

// timeNowUTC returns the current UTC timestamp.
func timeNowUTC() string {
	return timeNow().UTC().Format(time.RFC3339)
}

// timeNow returns the current time.
func timeNow() time.Time {
	return time.Now()
}

// Payment returns the payment service info resolver.
func PaymentServiceInfo_Resolver(ctx context.Context, args struct{}) (map[string]interface{}, error) {
	resolver := &Resolver{}
	return resolver.PaymentServiceInfo(ctx)
}

// GetPaymentResolver returns the payment resolver.
func GetPaymentResolver(ctx context.Context, id string) (*Payment, error) {
	resolver := &Resolver{}
	return resolver.Payment(ctx, id)
}