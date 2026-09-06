package ports

import (
	"context"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

// AssignmentRepository defines the interface for assignment data access.
type AssignmentRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Assignment, error)
	FindActiveByDriver(ctx context.Context, driverID string) (string, bool)
	FindByDeliveryID(ctx context.Context, deliveryID string) (string, bool)
	Save(ctx context.Context, assignmentID string, driverID string, deliveryID string, status string) error
	UpdateStatus(ctx context.Context, assignmentID string, status string) error
	ExpireOffers(ctx context.Context, olderThan time.Duration) ([]string, error)
}