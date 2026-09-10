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

// AssignmentRepository implements data access for assignments using MongoDB.
type AssignmentRepository struct {
	client     *mongo.Client
	database   string
	collection string
}

// NewAssignmentRepository creates a new AssignmentRepository.
func NewAssignmentRepository(client *mongo.Client, database, collection string) *AssignmentRepository {
	return &AssignmentRepository{
		client:     client,
		database:   database,
		collection: collection,
	}
}

// FindByID retrieves an assignment by ID.
func (r *AssignmentRepository) FindByID(ctx context.Context, id string) (*domain.Assignment, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	var result domain.Assignment
	err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

// FindActiveByDriver finds the active/offered assignment for a driver.
func (r *AssignmentRepository) FindActiveByDriver(ctx context.Context, driverID string) (*domain.Assignment, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	var result domain.Assignment
	err := col.FindOne(ctx, bson.M{
		"$or": []bson.M{{"driverId": driverID}, {"driverid": driverID}},
		"status":   bson.M{"$in": []string{"OFFERED", "ACCEPTED", "ACTIVE"}},
	}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

// FindByDeliveryID finds assignment by delivery ID.
func (r *AssignmentRepository) FindByDeliveryID(ctx context.Context, deliveryID string) (*domain.Assignment, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	var result domain.Assignment
	err := col.FindOne(ctx, bson.M{
		"$or": []bson.M{{"deliveryId": deliveryID}, {"deliveryid": deliveryID}},
	}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

// Save saves or updates an assignment (upsert by ID).
func (r *AssignmentRepository) Save(ctx context.Context, assignment *domain.Assignment) error {
	col := r.client.Database(r.database).Collection(r.collection)
	_, err := col.ReplaceOne(ctx, bson.M{"_id": assignment.ID}, assignment, options.Replace().SetUpsert(true))
	return err
}

// UpdateStatus updates assignment status.
func (r *AssignmentRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	col := r.client.Database(r.database).Collection(r.collection)
	_, err := col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": status, "updatedAt": time.Now()}})
	return err
}

// ExpireOffers marks OFFERED assignments older than the given duration as EXPIRED.
func (r *AssignmentRepository) ExpireOffers(ctx context.Context, olderThan time.Duration) ([]string, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	cutoff := time.Now().Add(-olderThan)
	cursor, err := col.Find(ctx, bson.M{
		"status":    "OFFERED",
		"expiresAt": bson.M{"$lt": cutoff},
	})
	if err != nil {
		return nil, err
	}
	var assignments []*domain.Assignment
	if err = cursor.All(ctx, &assignments); err != nil {
		return nil, err
	}
	var expiredIDs []string
	for _, a := range assignments {
		expiredIDs = append(expiredIDs, a.ID)
		_, _ = col.UpdateOne(ctx, bson.M{"_id": a.ID}, bson.M{
			"$set": bson.M{"status": "EXPIRED", "updatedAt": time.Now()},
		})
	}
	return expiredIDs, nil
}

// EnsureIndexes creates indexes for common query patterns.
func (r *AssignmentRepository) EnsureIndexes(ctx context.Context) error {
	col := r.client.Database(r.database).Collection(r.collection)
	indexModels := []mongo.IndexModel{
		{Keys: bson.D{{Key: "driverId", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "deliveryId", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}}},
		{Keys: bson.D{{Key: "createdAt", Value: -1}}},
	}
	_, err := col.Indexes().CreateMany(ctx, indexModels)
	return err
}

// Compile-time interface check
var _ ports.AssignmentRepository = (*AssignmentRepository)(nil)