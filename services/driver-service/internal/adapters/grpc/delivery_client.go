package grpc

import (
	"context"

	pb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc/proto"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

type deliveryServiceClientAdapter struct {
	client pb.DeliveryServiceClient
}

// NewDeliveryServiceClientAdapter creates a new ports.DeliveryServiceClient adapter using gRPC.
func NewDeliveryServiceClientAdapter(client pb.DeliveryServiceClient) ports.DeliveryServiceClient {
	return &deliveryServiceClientAdapter{client: client}
}

func (a *deliveryServiceClientAdapter) GetDelivery(ctx context.Context, deliveryID string) (*ports.DeliveryInfo, error) {
	resp, err := a.client.GetDelivery(ctx, &pb.GetDeliveryRequest{
		DeliveryId: deliveryID,
	})
	if err != nil {
		return nil, err
	}
	return &ports.DeliveryInfo{
		DeliveryID: resp.DeliveryId,
		CustomerID: resp.CustomerId,
		DriverID:   resp.DriverId,
		Status:     resp.Status,
		Amount:     resp.Amount,
		Currency:   resp.Currency,
	}, nil
}

func (a *deliveryServiceClientAdapter) GetOpenDeliveries(ctx context.Context, page, limit int32) ([]*ports.OpenDeliveryInfo, int32, error) {
	resp, err := a.client.GetOpenDeliveries(ctx, &pb.GetOpenDeliveriesRequest{
		Page:  page,
		Limit: limit,
	})
	if err != nil {
		return nil, 0, err
	}

	result := make([]*ports.OpenDeliveryInfo, 0, len(resp.Items))
	for _, item := range resp.Items {
		result = append(result, &ports.OpenDeliveryInfo{
			ID:             item.Id,
			CustomerID:     item.CustomerId,
			Status:         item.Status,
			PickupCity:     item.PickupCity,
			PickupCountry:  item.PickupCountry,
			DropoffCity:    item.DropoffCity,
			DropoffCountry: item.DropoffCountry,
			Amount:         item.Amount,
			Currency:       item.Currency,
			CreatedAt:      item.CreatedAt,
		})
	}
	return result, resp.TotalItems, nil
}

