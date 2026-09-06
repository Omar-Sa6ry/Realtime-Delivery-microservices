package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

// DriverRepository implements data access for drivers using MongoDB.
type DriverRepository struct {
	client   *mongo.Client
	database string
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

// FindAvailableByLocation finds available drivers near given coordinates.
func (r *DriverRepository) FindAvailableByLocation(ctx context.Context, lat, lng, radiusKm float64) ([]*domain.Driver, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	cursor, err := col.Find(ctx, bson.M{"status": "AVAILABLE"})
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

// Save saves or updates a driver.
func (r *DriverRepository) Save(ctx context.Context, driver *domain.Driver) error {
	col := r.client.Database(r.database).Collection(r.collection)
	_, err := col.ReplaceOne(ctx, bson.M{"_id": driver.ID}, driver, options.Replace().SetUpsert(true))
	return err
}