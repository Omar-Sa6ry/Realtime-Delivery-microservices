package ports

import "context"

// DeliveryInfo represents basic delivery information required by driver service.
type DeliveryInfo struct {
	DeliveryID string
	CustomerID string
	DriverID   string
	Status     string
	Amount     float64
	Currency   string
}

// DeliveryServiceClient defines the port for interacting with the delivery-service.
type DeliveryServiceClient interface {
	GetDelivery(ctx context.Context, deliveryID string) (*DeliveryInfo, error)
}
