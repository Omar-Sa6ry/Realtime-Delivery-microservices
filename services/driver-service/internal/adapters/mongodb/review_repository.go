package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// ReviewRepository implements ReviewRepository interface using MongoDB.
type ReviewRepository struct {
	client     *mongo.Client
	database   string
	collection string
}

// NewReviewRepository creates a new ReviewRepository.
func NewReviewRepository(client *mongo.Client, database, collection string) *ReviewRepository {
	return &ReviewRepository{
		client:     client,
		database:   database,
		collection: collection,
	}
}

// Create saves a new review in MongoDB.
func (r *ReviewRepository) Create(ctx context.Context, review *domain.Review) error {
	col := r.client.Database(r.database).Collection(r.collection)
	_, err := col.InsertOne(ctx, review)
	return err
}

// FindByDriverID retrieves reviews for a given driver with pagination.
func (r *ReviewRepository) FindByDriverID(ctx context.Context, driverID string, skip, limit int) ([]*domain.Review, int64, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	filter := bson.M{"driverid": driverID} // using lower case as mongo driver serializes struct tags

	total, err := col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.M{"createdat": -1}) // sort descending by creation date

	cursor, err := col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var reviews []*domain.Review
	if err = cursor.All(ctx, &reviews); err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

// FindByDeliveryID retrieves a review by delivery ID.
func (r *ReviewRepository) FindByDeliveryID(ctx context.Context, deliveryID string) (*domain.Review, error) {
	col := r.client.Database(r.database).Collection(r.collection)
	var review domain.Review
	err := col.FindOne(ctx, bson.M{"deliveryid": deliveryID}).Decode(&review)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // return nil if not found
		}
		return nil, err
	}
	return &review, nil
}

// CalculateAverageRating computes the average rating and count for a driver using an aggregation pipeline.
func (r *ReviewRepository) CalculateAverageRating(ctx context.Context, driverID string) (float64, int64, error) {
	col := r.client.Database(r.database).Collection(r.collection)

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"driverid": driverID}}},
		{{Key: "$group", Value: bson.M{
			"_id":           nil,
			"averageRating": bson.M{"$avg": "$rating"},
			"totalReviews":  bson.M{"$sum": 1},
		}}},
	}

	cursor, err := col.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, 0, err
	}
	defer cursor.Close(ctx)

	if !cursor.Next(ctx) {
		// No reviews found
		return 0, 0, nil
	}

	var result struct {
		AverageRating float64 `bson:"averageRating"`
		TotalReviews  int64   `bson:"totalReviews"`
	}

	if err := cursor.Decode(&result); err != nil {
		return 0, 0, err
	}

	return result.AverageRating, result.TotalReviews, nil
}

// EnsureIndexes creates indexes for common query patterns.
func (r *ReviewRepository) EnsureIndexes(ctx context.Context) error {
	col := r.client.Database(r.database).Collection(r.collection)
	indexModels := []mongo.IndexModel{
		{Keys: bson.D{{Key: "driverid", Value: 1}, {Key: "createdat", Value: -1}}},
		{Keys: bson.D{{Key: "deliveryid", Value: 1}}, Options: options.Index().SetUnique(true)},
	}
	_, err := col.Indexes().CreateMany(ctx, indexModels)
	return err
}

// Compile-time interface check
var _ ports.ReviewRepository = (*ReviewRepository)(nil)
