package graphql

import (
	"context"
	"time"
)

// Resolver is the GraphQL resolver root
type Resolver struct{}

// PaymentServiceInfo returns service health info
func (r *Resolver) PaymentServiceInfo(ctx context.Context) (map[string]interface{}, error) {
	now := time.Now().UTC().Format(time.RFC3339)
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