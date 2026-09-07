package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// DispatchAttemptRepository implements data access for dispatch attempts using MongoDB.
type DispatchAttemptRepository struct {
	client     *mongo.Client
	database   string
	collection string
}

// NewDispatchAttemptRepository creates a new DispatchAttemptRepository.
func NewDispatchAttemptRepository(client *mongo.Client, database, collection string) *DispatchAttemptRepository {
	return &DispatchAttemptRepository{
		client:     client,
		database:   database,
		collection: collection,
	}
}

// Save saves a dispatch attempt.
func (r *DispatchAttemptRepository) Save(ctx context.Context, attempt *domain.DispatchAttempt) error {
	col := r.client.Database(r.database).Collection(r.collection)
	_, err := col.InsertOne(ctx, attempt)
	return err
}

// FindByDeliveryID finds dispatch attempts by delivery ID.
func (r *DispatchAttemptRepository) FindByDeliveryID(ctx context.Context, deliveryID string) ([]*domain.DispatchAttempt, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	cursor, err := col.Find(ctx, bson.M{"deliveryId": deliveryID})
	if err != nil {
		return nil, err
	}
	var attempts []*domain.DispatchAttempt
	if err = cursor.All(ctx, &attempts); err != nil {
		return nil, err
	}
	return attempts, nil
}

// FindByDriverID finds dispatch attempts by driver ID.
func (r *DispatchAttemptRepository) FindByDriverID(ctx context.Context, driverID string) ([]*domain.DispatchAttempt, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	cursor, err := col.Find(ctx, bson.M{"driverId": driverID})
	if err != nil {
		return nil, err
	}
	var attempts []*domain.DispatchAttempt
	if err = cursor.All(ctx, &attempts); err != nil {
		return nil, err
	}
	return attempts, nil
}

// CountByDeliveryID counts dispatch attempts for a delivery.
func (r *DispatchAttemptRepository) CountByDeliveryID(ctx context.Context, deliveryID string) (int64, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	return col.CountDocuments(ctx, bson.M{"deliveryId": deliveryID})
}

// EnsureIndex creates the necessary indexes.
func (r *DispatchAttemptRepository) EnsureIndex(ctx context.Context) error {
	col := r.client.Database(r.database).Collection(r.collection)
	indexModels := []mongo.IndexModel{
		{Keys: bson.D{{Key: "deliveryId", Value: 1}}},
		{Keys: bson.D{{Key: "driverId", Value: 1}}},
		{Keys: bson.D{{Key: "createdAt", Value: 1}}},
	}
	_, err := col.Indexes().CreateMany(ctx, indexModels)
	return err
}

// Compile-time interface check
var _ ports.DispatchAttemptRepository = (*DispatchAttemptRepository)(nil)