package ports

import "context"

type GeoDriverResult struct {
	DriverID       string
	DistanceMeters float64
	Longitude      float64
	Latitude       float64
}

type LocationStore interface {
	GEOAdd(ctx context.Context, longitude, latitude, driverID string) error
	GEOSearch(ctx context.Context, longitude, latitude, radiusKm float64) error
	GEOSearchDrivers(ctx context.Context, longitude, latitude, radiusKm float64) ([]GeoDriverResult, error)
	NearbyDriversSorted(ctx context.Context, longitude, latitude, radiusKm float64) ([]GeoDriverResult, error)
	GetDriverLocation(ctx context.Context, driverID string) (string, string, error)
}