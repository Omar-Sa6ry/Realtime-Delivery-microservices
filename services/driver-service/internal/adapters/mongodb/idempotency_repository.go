package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// IdempotencyRepository implements idempotency store using MongoDB.
type IdempotencyRepository struct {
	client     *mongo.Client
	database   string
	collection string
}

// NewIdempotencyRepository creates a new IdempotencyRepository.
func NewIdempotencyRepository(client *mongo.Client, database, collection string) *IdempotencyRepository {
	return &IdempotencyRepository{
		client:     client,
		database:   database,
		collection: collection,
	}
}

// CheckAndStore checks if a key exists and stores the result atomically.
func (r *IdempotencyRepository) CheckAndStore(ctx context.Context, key string, result []byte, ttl int) (bool, error) {
	col := r.client.Database(r.database).Collection(r.collection)

	filter := bson.M{"_id": key}
	update := bson.M{
		"$setOnInsert": bson.M{
			"_id":        key,
			"result":     result,
			"createdAt":  time.Now(),
			"expiresAt":  time.Now().Add(time.Duration(ttl) * time.Second),
		},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var doc bson.M
	err := col.FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return true, nil // Key was inserted (first time)
	}
	if err != nil {
		return false, err
	}
	return false, nil // Key already existed
}

// Exists checks if a key exists.
func (r *IdempotencyRepository) Exists(ctx context.Context, key string) (bool, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	count, err := col.CountDocuments(ctx, bson.M{"_id": key})
	return count > 0, err
}

// Delete removes an idempotency key.
func (r *IdempotencyRepository) Delete(ctx context.Context, key string) error {
	col := r.client.Database(r.database).Collection(r.collection)
	_, err := col.DeleteOne(ctx, bson.M{"_id": key})
	return err
}

// CleanupExpired removes expired idempotency keys.
func (r *IdempotencyRepository) CleanupExpired(ctx context.Context) (int64, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	result, err := col.DeleteMany(ctx, bson.M{"expiresAt": bson.M{"$lt": time.Now()}})
	return result.DeletedCount, err
}

// EnsureIndex creates the necessary indexes.
func (r *IdempotencyRepository) EnsureIndex(ctx context.Context) error {
	col := r.client.Database(r.database).Collection(r.collection)
	indexModels := []mongo.IndexModel{
		{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	}
	_, err := col.Indexes().CreateMany(ctx, indexModels)
	return err
}

// Compile-time interface check
var _ ports.IdempotencyStore = (*IdempotencyRepository)(nil)