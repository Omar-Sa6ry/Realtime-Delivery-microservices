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

// OpenDeliveryInfo represents open/unassigned delivery summary for drivers to browse.
type OpenDeliveryInfo struct {
	ID             string
	CustomerID     string
	Status         string
	PickupCity     string
	PickupCountry  string
	DropoffCity    string
	DropoffCountry string
	Amount         string
	Currency       string
	CreatedAt      string
}

// DeliveryServiceClient defines the port for interacting with the delivery-service.
type DeliveryServiceClient interface {
	GetDelivery(ctx context.Context, deliveryID string) (*DeliveryInfo, error)
	GetOpenDeliveries(ctx context.Context, page, limit int32) ([]*OpenDeliveryInfo, int32, error)
}

