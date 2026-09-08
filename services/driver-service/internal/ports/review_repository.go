package ports

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

// ReviewRepository defines the interface for Review data access.
type ReviewRepository interface {
	// Create saves a new review.
	Create(ctx context.Context, review *domain.Review) error

	// FindByDriverID retrieves reviews for a driver with pagination.
	FindByDriverID(ctx context.Context, driverID string, skip, limit int) ([]*domain.Review, int64, error)

	// FindByDeliveryID retrieves a review by delivery ID.
	FindByDeliveryID(ctx context.Context, deliveryID string) (*domain.Review, error)

	// CalculateAverageRating computes the average rating and count for a driver.
	CalculateAverageRating(ctx context.Context, driverID string) (float64, int64, error)
}
