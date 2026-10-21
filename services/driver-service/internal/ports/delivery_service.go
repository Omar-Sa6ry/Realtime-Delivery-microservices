package ports

import "context"

type DeliveryInfo struct {
	DeliveryID string
	CustomerID string
	DriverID   string
	Status     string
	Amount     float64
	Currency   string
}

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

type DeliveryServiceClient interface {
	GetDelivery(ctx context.Context, deliveryID string) (*DeliveryInfo, error)
	GetOpenDeliveries(ctx context.Context, page, limit int32) ([]*OpenDeliveryInfo, int32, error)
}

