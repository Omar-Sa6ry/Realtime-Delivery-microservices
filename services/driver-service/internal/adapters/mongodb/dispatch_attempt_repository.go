package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

type DispatchAttemptRepository struct {
	client     *mongo.Client
	database   string
	collection string
}

func NewDispatchAttemptRepository(client *mongo.Client, database, collection string) *DispatchAttemptRepository {
	return &DispatchAttemptRepository{
		client:     client,
		database:   database,
		collection: collection,
	}
}

func (r *DispatchAttemptRepository) Save(ctx context.Context, attempt *domain.DispatchAttempt) error {
	col := r.client.Database(r.database).Collection(r.collection)
	_, err := col.InsertOne(ctx, attempt)
	return err
}

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

func (r *DispatchAttemptRepository) CountByDeliveryID(ctx context.Context, deliveryID string) (int64, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	return col.CountDocuments(ctx, bson.M{"deliveryId": deliveryID})
}

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

var _ ports.DispatchAttemptRepository = (*DispatchAttemptRepository)(nil)