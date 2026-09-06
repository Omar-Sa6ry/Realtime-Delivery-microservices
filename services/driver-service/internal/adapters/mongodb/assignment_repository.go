package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
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
		return nil, err
	}
	return &result, nil
}

// FindActiveByDriver finds active assignment for a driver.
func (r *AssignmentRepository) FindActiveByDriver(ctx context.Context, driverID string) (*domain.Assignment, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	var result domain.Assignment
	err := col.FindOne(ctx, bson.M{"driverId": driverID, "status": "ACTIVE"}).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// FindByDeliveryID finds assignment by delivery ID.
func (r *AssignmentRepository) FindByDeliveryID(ctx context.Context, deliveryID string) (*domain.Assignment, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	var result domain.Assignment
	err := col.FindOne(ctx, bson.M{"deliveryId": deliveryID}).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Save saves or updates an assignment.
func (r *AssignmentRepository) Save(ctx context.Context, assignment *domain.Assignment) error {
	col := r.client.Database(r.database).Collection(r.collection)
	_, err := col.ReplaceOne(ctx, bson.M{"_id": assignment.ID}, assignment, options.Replace().SetUpsert(true))
	return err
}

// UpdateStatus updates assignment status.
func (r *AssignmentRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	col := r.client.Database(r.database).Collection(r.collection)
	_, err := col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": status}})
	return err
}