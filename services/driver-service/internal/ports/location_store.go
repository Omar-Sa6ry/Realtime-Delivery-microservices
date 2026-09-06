package ports

import "context"

// LocationStore defines the interface for Redis GEO operations.
type LocationStore interface {
	GEOAdd(ctx context.Context, longitude, latitude, driverID string) error
	GEOSearch(ctx context.Context, longitude, latitude, radiusKm float64) error
	GetDriverLocation(ctx context.Context, driverID string) (string, string, error)
}