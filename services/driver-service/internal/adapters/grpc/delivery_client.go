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
