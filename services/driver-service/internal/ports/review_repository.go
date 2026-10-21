package ports

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

type ReviewRepository interface {
	Create(ctx context.Context, review *domain.Review) error
	FindByDriverID(ctx context.Context, driverID string, skip, limit int) ([]*domain.Review, int64, error)
	FindByDeliveryID(ctx context.Context, deliveryID string) (*domain.Review, error)
	CalculateAverageRating(ctx context.Context, driverID string) (float64, int64, error)
}
