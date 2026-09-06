package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// GeoStore implements geo operations using Redis.
type GeoStore struct {
	client *redis.Client
}

// NewGeoStore creates a new GeoStore.
func NewGeoStore(client *redis.Client) *GeoStore {
	return &GeoStore{client: client}
}

// SetDriverLocation stores a driver's location.
func (g *GeoStore) SetDriverLocation(ctx context.Context, driverID, latitude, longitude string) error {
	key := fmt.Sprintf("driver:location:%s", driverID)
	return g.client.Set(ctx, key, fmt.Sprintf(`{"lat":%s,"lng":%s}`, latitude, longitude), 0).Err()
}

// GetDriverLocation retrieves a driver's current location.
func (g *GeoStore) GetDriverLocation(ctx context.Context, driverID string) (string, string, error) {
	key := fmt.Sprintf("driver:location:%s", driverID)
	val, err := g.client.Get(ctx, key).Result()
	if err != nil {
		return "", "", err
	}
	// Simple parsing - return the stored string
	return val, "", nil
}