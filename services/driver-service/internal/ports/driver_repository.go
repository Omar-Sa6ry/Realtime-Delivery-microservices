package ports

import (
	"context"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

// DriverRepository defines the interface for driver data access.
type DriverRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Driver, error)
	FindAvailableByLocation(ctx context.Context, lat, lng, radiusKm float64, vehicleType string) ([]*domain.Driver, error)
	FindByStatus(ctx context.Context, status string) ([]*domain.Driver, error)
	Save(ctx context.Context, driver *domain.Driver) error
}