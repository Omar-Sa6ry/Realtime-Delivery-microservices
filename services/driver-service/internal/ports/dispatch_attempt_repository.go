package ports

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

// DispatchAttemptRepository defines the interface for dispatch attempt data access.
type DispatchAttemptRepository interface {
	Save(ctx context.Context, attempt *domain.DispatchAttempt) error
	FindByDeliveryID(ctx context.Context, deliveryID string) ([]*domain.DispatchAttempt, error)
	FindByDriverID(ctx context.Context, driverID string) ([]*domain.DispatchAttempt, error)
	CountByDeliveryID(ctx context.Context, deliveryID string) (int64, error)
	EnsureIndex(ctx context.Context) error
}