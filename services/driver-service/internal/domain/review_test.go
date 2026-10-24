package domain_test

import (
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

func TestNewReview(t *testing.T) {
	tests := []struct {
		name          string
		driverID      string
		userID        string
		deliveryID    string
		rating        float64
		comment       string
		expectedError error
	}{
		{
			name:          "valid review",
			driverID:      "driver-1",
			userID:        "user-1",
			deliveryID:    "delivery-1",
			rating:        4.5,
			comment:       "Great service!",
			expectedError: nil,
		},
		{
			name:          "empty driverID",
			driverID:      "",
			userID:        "user-1",
			deliveryID:    "delivery-1",
			rating:        4.5,
			comment:       "Great service!",
			expectedError: domain.ErrInvalidArgument,
		},
		{
			name:          "empty userID",
			driverID:      "driver-1",
			userID:        "",
			deliveryID:    "delivery-1",
			rating:        4.5,
			comment:       "Great service!",
			expectedError: domain.ErrInvalidArgument,
		},
		{
			name:          "empty deliveryID",
			driverID:      "driver-1",
			userID:        "user-1",
			deliveryID:    "",
			rating:        4.5,
			comment:       "Great service!",
			expectedError: domain.ErrInvalidArgument,
		},
		{
			name:          "rating too low",
			driverID:      "driver-1",
			userID:        "user-1",
			deliveryID:    "delivery-1",
			rating:        0.5,
			comment:       "Bad service",
			expectedError: domain.ErrInvalidRating,
		},
		{
			name:          "rating too high",
			driverID:      "driver-1",
			userID:        "user-1",
			deliveryID:    "delivery-1",
			rating:        5.5,
			comment:       "Amazing!",
			expectedError: domain.ErrInvalidRating,
		},
		{
			name:          "rating exactly 1.0",
			driverID:      "driver-1",
			userID:        "user-1",
			deliveryID:    "delivery-1",
			rating:        1.0,
			comment:       "Ok",
			expectedError: nil,
		},
		{
			name:          "rating exactly 5.0",
			driverID:      "driver-1",
			userID:        "user-1",
			deliveryID:    "delivery-1",
			rating:        5.0,
			comment:       "Perfect!",
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			review, err := domain.NewReview(tt.driverID, tt.userID, tt.deliveryID, tt.rating, tt.comment)
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("NewReview() expected error %v, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("NewReview() error = %v, want %v", err, tt.expectedError)
				}
				if review != nil {
					t.Errorf("NewReview() should return nil review on error")
				}
			} else {
				if err != nil {
					t.Errorf("NewReview() unexpected error: %v", err)
				}
				if review == nil {
					t.Fatal("NewReview() returned nil review")
				}
				if review.DriverID != tt.driverID {
					t.Errorf("DriverID = %v, want %v", review.DriverID, tt.driverID)
				}
				if review.UserID != tt.userID {
					t.Errorf("UserID = %v, want %v", review.UserID, tt.userID)
				}
				if review.DeliveryID != tt.deliveryID {
					t.Errorf("DeliveryID = %v, want %v", review.DeliveryID, tt.deliveryID)
				}
				if review.Rating != tt.rating {
					t.Errorf("Rating = %v, want %v", review.Rating, tt.rating)
				}
				if review.Comment != tt.comment {
					t.Errorf("Comment = %v, want %v", review.Comment, tt.comment)
				}
				if review.ID == "" {
					t.Errorf("ID should be generated")
				}
				if review.CreatedAt.IsZero() {
					t.Errorf("CreatedAt should be set")
				}
				if review.UpdatedAt.IsZero() {
					t.Errorf("UpdatedAt should be set")
				}
			}
		})
	}
}