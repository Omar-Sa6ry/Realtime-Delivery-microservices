package queries_test

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/queries"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

func TestGetDriverReviewsHandler_Execute(t *testing.T) {
	tests := []struct {
		name           string
		setupDriver    *domain.Driver
		setupReviews   []*domain.Review
		page           int
		limit          int
		expectedError  error
		expectedCount  int
		expectedTotal  int64
	}{
		{
			name: "success - reviews found",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
				Rating:    4.5,
				RatingCount: 2,
			},
			setupReviews: []*domain.Review{
				{ID: "review-1", DriverID: "driver-1", UserID: "user-1", DeliveryID: "delivery-1", Rating: 5.0},
				{ID: "review-2", DriverID: "driver-1", UserID: "user-2", DeliveryID: "delivery-2", Rating: 4.0},
			},
			page: 1,
			limit: 10,
			expectedError: nil,
			expectedCount: 2,
			expectedTotal: 2,
		},
		{
			name: "success - empty reviews",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
				Rating:    0,
				RatingCount: 0,
			},
			setupReviews: []*domain.Review{},
			page: 1,
			limit: 10,
			expectedError: nil,
			expectedCount: 0,
			expectedTotal: 0,
		},
		{
			name: "fails - driver not found",
			setupDriver: nil,
			setupReviews: []*domain.Review{},
			page: 1,
			limit: 10,
			expectedError: domain.ErrDriverNotFound,
			expectedCount: 0,
			expectedTotal: 0,
		},
		{
			name: "success - pagination",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
				Rating:    4.5,
				RatingCount: 5,
			},
			setupReviews: []*domain.Review{
				{ID: "review-1", DriverID: "driver-1", Rating: 5.0},
				{ID: "review-2", DriverID: "driver-1", Rating: 4.0},
				{ID: "review-3", DriverID: "driver-1", Rating: 3.0},
				{ID: "review-4", DriverID: "driver-1", Rating: 5.0},
				{ID: "review-5", DriverID: "driver-1", Rating: 4.0},
			},
			page: 2,
			limit: 2,
			expectedError: nil,
			expectedCount: 2,
			expectedTotal: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviewRepo := mocks.NewMockReviewRepository()
			driverRepo := mocks.NewMockDriverRepository()

			if tt.setupDriver != nil {
				driverRepo.SetDriver(tt.setupDriver)
			}
			for _, review := range tt.setupReviews {
				reviewRepo.SetReview(review)
			}

			handler := queries.NewGetDriverReviewsHandler(reviewRepo, driverRepo)

			query := queries.GetDriverReviewsQuery{
				DriverID: "driver-1",
				Page:     tt.page,
				Limit:    tt.limit,
			}

			result, err := handler.Execute(context.Background(), query)

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Execute() expected error %v, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("Execute() error = %v, want %v", err, tt.expectedError)
				}
				if result != nil {
					t.Errorf("Execute() should return nil result on error")
				}
			} else {
				if err != nil {
					t.Errorf("Execute() unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("Execute() returned nil result")
				}
				if len(result.Reviews) != tt.expectedCount {
					t.Errorf("Result.Reviews count = %v, want %v", len(result.Reviews), tt.expectedCount)
				}
				if result.TotalItems != tt.expectedTotal {
					t.Errorf("Result.TotalItems = %v, want %v", result.TotalItems, tt.expectedTotal)
				}
				if result.AverageRating != tt.setupDriver.Rating {
					t.Errorf("Result.AverageRating = %v, want %v", result.AverageRating, tt.setupDriver.Rating)
				}
				if result.TotalReviews != tt.setupDriver.RatingCount {
					t.Errorf("Result.TotalReviews = %v, want %v", result.TotalReviews, tt.setupDriver.RatingCount)
				}
				if result.CurrentPage != tt.page {
					t.Errorf("Result.CurrentPage = %v, want %v", result.CurrentPage, tt.page)
				}
				if result.Limit != tt.limit {
					t.Errorf("Result.Limit = %v, want %v", result.Limit, tt.limit)
				}
			}
		})
	}
}