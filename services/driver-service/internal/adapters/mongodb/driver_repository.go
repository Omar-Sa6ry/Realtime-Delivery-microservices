package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// DriverRepository implements data access for drivers using MongoDB.
type DriverRepository struct {
	client     *mongo.Client
	database   string
	collection string
}

// NewDriverRepository creates a new DriverRepository.
func NewDriverRepository(client *mongo.Client, database, collection string) *DriverRepository {
	return &DriverRepository{
		client:     client,
		database:   database,
		collection: collection,
	}
}

// FindByID retrieves a driver by ID.
func (r *DriverRepository) FindByID(ctx context.Context, id string) (*domain.Driver, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	var result domain.Driver
	err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// FindByUserID retrieves a driver by linked user ID (for gRPC validation).
func (r *DriverRepository) FindByUserID(ctx context.Context, userID string) (*domain.Driver, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	var result domain.Driver
	err := col.FindOne(ctx, bson.M{"userId": userID}).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// FindAvailableByLocation finds available drivers near given coordinates,
// optionally filtered by vehicle type.
func (r *DriverRepository) FindAvailableByLocation(ctx context.Context, lat, lng, radiusKm float64, vehicleType string) ([]*domain.Driver, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	filter := bson.M{"status": "AVAILABLE"}
	if vehicleType != "" {
		filter["vehicle.type"] = vehicleType
	}
	cursor, err := col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var drivers []*domain.Driver
	if err = cursor.All(ctx, &drivers); err != nil {
		return nil, err
	}
	return drivers, nil
}

// FindByStatus finds drivers by status.
func (r *DriverRepository) FindByStatus(ctx context.Context, status string) ([]*domain.Driver, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	cursor, err := col.Find(ctx, bson.M{"status": status})
	if err != nil {
		return nil, err
	}
	var drivers []*domain.Driver
	if err = cursor.All(ctx, &drivers); err != nil {
		return nil, err
	}
	return drivers, nil
}

// Save saves or updates a driver (upsert by ID).
func (r *DriverRepository) Save(ctx context.Context, driver *domain.Driver) error {
	col := r.client.Database(r.database).Collection(r.collection)
	_, err := col.ReplaceOne(ctx, bson.M{"_id": driver.ID}, driver, options.Replace().SetUpsert(true))
	return err
}

// EnsureIndexes creates indexes for common query patterns.
func (r *DriverRepository) EnsureIndexes(ctx context.Context) error {
	col := r.client.Database(r.database).Collection(r.collection)
	indexModels := []mongo.IndexModel{
		{Keys: bson.D{{Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "userId", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "vehicle.type", Value: 1}}},
		{Keys: bson.D{{Key: "updatedAt", Value: -1}}},
	}
	_, err := col.Indexes().CreateMany(ctx, indexModels)
	return err
}

// DeleteExpiredLocations removes stale location entries (used by reconciliation).
func (r *DriverRepository) DeleteExpiredLocations(ctx context.Context, olderThan time.Duration) (int64, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	cutoff := time.Now().Add(-olderThan)
	result, err := col.UpdateMany(
		ctx,
		bson.M{"status": "AVAILABLE", "updatedAt": bson.M{"$lt": cutoff}},
		bson.M{"$set": bson.M{"status": "OFFLINE"}},
	)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

// Compile-time interface check
var _ ports.DriverRepository = (*DriverRepository)(nil)