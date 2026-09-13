package graphql

import (
	"context"
	"time"
)

// Resolver is the GraphQL resolver root for payment operations.
type Resolver struct{}

// NewResolver creates a new GraphQL resolver.
func NewResolver() *Resolver {
	return &Resolver{}
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

// timeNowUTC returns the current UTC timestamp.
func timeNowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}