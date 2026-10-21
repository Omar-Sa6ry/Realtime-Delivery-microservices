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

type DriverRepository struct {
	client     *mongo.Client
	database   string
	collection string
}

func NewDriverRepository(client *mongo.Client, database, collection string) *DriverRepository {
	return &DriverRepository{
		client:     client,
		database:   database,
		collection: collection,
	}
}

func (r *DriverRepository) FindByID(ctx context.Context, id string) (*domain.Driver, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	var result domain.Driver
	err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

func (r *DriverRepository) FindByIDs(ctx context.Context, ids []string) ([]*domain.Driver, error) {
	if len(ids) == 0 {
		return []*domain.Driver{}, nil
	}
	col := r.client.Database(r.database).Collection(r.collection)
	cursor, err := col.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	var drivers []*domain.Driver
	if err = cursor.All(ctx, &drivers); err != nil {
		return nil, err
	}
	return drivers, nil
}

func (r *DriverRepository) FindByUserID(ctx context.Context, userID string) (*domain.Driver, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	var result domain.Driver
	err := col.FindOne(ctx, bson.M{"$or": []bson.M{{"userId": userID}, {"userid": userID}}}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

func (r *DriverRepository) FindAvailableByLocation(ctx context.Context, lat, lng, radiusKm float64, vehicleType domain.VehicleType) ([]*domain.Driver, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	filter := bson.M{"status": "AVAILABLE", "isBlocked": false, "isActive": true}
	if vehicleType != "" {
		filter["$or"] = []bson.M{
			{"vehicle.type": vehicleType},
			{"vehicle.type": string(vehicleType)},
		}
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

func (r *DriverRepository) Save(ctx context.Context, driver *domain.Driver) error {
	col := r.client.Database(r.database).Collection(r.collection)
	_, err := col.ReplaceOne(ctx, bson.M{"_id": driver.ID}, driver, options.Replace().SetUpsert(true))
	return err
}

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

func (r *DriverRepository) UpdateRating(ctx context.Context, driverID string, averageRating float64, totalReviews int64) error {
	col := r.client.Database(r.database).Collection(r.collection)
	update := bson.M{
		"$set": bson.M{
			"rating":      averageRating,
			"ratingcount": totalReviews,
			"updatedat":   time.Now(),
		},
	}
	_, err := col.UpdateByID(ctx, driverID, update)
	return err
}

// Compile-time interface check
var _ ports.DriverRepository = (*DriverRepository)(nil)