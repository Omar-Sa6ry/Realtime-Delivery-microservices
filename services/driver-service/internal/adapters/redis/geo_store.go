package redis

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

const geoKey = "drivers:locations"

type GeoStore struct {
	client *redis.Client
}

func NewGeoStore(client *redis.Client) *GeoStore {
	return &GeoStore{client: client}
}

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

func (g *GeoStore) GEOSearchDrivers(ctx context.Context, longitude, latitude, radiusKm float64) ([]ports.GeoDriverResult, error) {
	results, err := g.client.GeoSearchLocation(ctx, geoKey, &redis.GeoSearchLocationQuery{
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
	if err != nil {
		return nil, err
	}

	driverResults := make([]ports.GeoDriverResult, 0, len(results))
	for _, loc := range results {
		driverResults = append(driverResults, ports.GeoDriverResult{
			DriverID:       loc.Name,
			DistanceMeters: loc.Dist * 1000.0, // redis GeoSearch with RadiusUnit "km" gives Dist in km
			Longitude:      loc.Longitude,
			Latitude:       loc.Latitude,
		})
	}
	return driverResults, nil
}

func (g *GeoStore) NearbyDriversSorted(ctx context.Context, longitude, latitude, radiusKm float64) ([]ports.GeoDriverResult, error) {
	return g.GEOSearchDrivers(ctx, longitude, latitude, radiusKm)
}

func (g *GeoStore) SetDriverLocation(ctx context.Context, driverID, latitude, longitude string) error {
	return g.GEOAdd(ctx, longitude, latitude, driverID)
}

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

func (g *GeoStore) RemoveDriver(ctx context.Context, driverID string) error {
	return g.client.ZRem(ctx, geoKey, driverID).Err()
}

// Compile-time interface check
var _ ports.LocationStore = (*GeoStore)(nil)