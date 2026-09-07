package redis

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

const geoKey = "drivers:locations"

// GeoStore implements geo operations using Redis GEO commands.
type GeoStore struct {
	client *redis.Client
}

// NewGeoStore creates a new GeoStore.
func NewGeoStore(client *redis.Client) *GeoStore {
	return &GeoStore{client: client}
}

// GEOAdd adds or updates a driver's location in the Redis GEO set.
func (g *GeoStore) GEOAdd(ctx context.Context, longitude, latitude, driverID string) error {
	lng, err := strconv.ParseFloat(longitude, 64)
	if err != nil {
		return fmt.Errorf("invalid longitude %q: %w", longitude, err)
	}
	lat, err := strconv.ParseFloat(latitude, 64)
	if err != nil {
		return fmt.Errorf("invalid latitude %q: %w", latitude, err)
	}
	return g.client.GeoAdd(ctx, geoKey, &redis.GeoLocation{
		Name:      driverID,
		Longitude: lng,
		Latitude:  lat,
	}).Err()
}

// GEOSearch searches for drivers within a radius of the given coordinates.
// returned location names separately via GEORADIUS if needed.
func (g *GeoStore) GEOSearch(ctx context.Context, longitude, latitude, radiusKm float64) error {
	_, err := g.client.GeoSearchLocation(ctx, geoKey, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  longitude,
			Latitude:   latitude,
			Radius:     radiusKm,
			RadiusUnit: "km",
			Sort:        "ASC",
		},
		WithCoord: true,
		WithDist:  true,
	}).Result()
	return err
}

// GEOSearchDrivers searches for drivers within a radius and returns their IDs with distances.
func (g *GeoStore) GEOSearchDrivers(ctx context.Context, longitude, latitude, radiusKm float64) ([]redis.GeoLocation, error) {
	return g.client.GeoSearchLocation(ctx, geoKey, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  longitude,
			Latitude:   latitude,
			Radius:     radiusKm,
			RadiusUnit: "km",
			Sort:        "ASC",
			Count:       100,
		},
		WithCoord: true,
		WithDist:  true,
	}).Result()
}

// SetDriverLocation stores a driver's location using GEO commands.
func (g *GeoStore) SetDriverLocation(ctx context.Context, driverID, latitude, longitude string) error {
	return g.GEOAdd(ctx, longitude, latitude, driverID)
}

// GetDriverLocation retrieves a driver's current location.
func (g *GeoStore) GetDriverLocation(ctx context.Context, driverID string) (string, string, error) {
	positions, err := g.client.GeoPos(ctx, geoKey, driverID).Result()
	if err != nil {
		return "", "", err
	}
	if len(positions) == 0 || positions[0] == nil {
		return "", "", fmt.Errorf("driver %s has no location", driverID)
	}
	lat := strconv.FormatFloat(positions[0].Latitude, 'f', 6, 64)
	lng := strconv.FormatFloat(positions[0].Longitude, 'f', 6, 64)
	return lat, lng, nil
}

// RemoveDriver removes a driver from the GEO set (e.g., when they go offline).
func (g *GeoStore) RemoveDriver(ctx context.Context, driverID string) error {
	return g.client.ZRem(ctx, geoKey, driverID).Err()
}

// Compile-time interface check
var _ ports.LocationStore = (*GeoStore)(nil)