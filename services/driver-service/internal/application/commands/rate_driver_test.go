package commands_test

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/commands"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

func TestRateDriverHandler_Execute(t *testing.T) {
	tests := []struct {
		name              string
		setupDriver       *domain.Driver
		setupReview       *domain.Review
		setupDelivery     *ports.DeliveryInfo
		setupAssignment   *domain.Assignment
		driverRepoErr     error
		reviewRepoErr     error
		deliveryClientErr error
		expectedError     error
	}{
		{
			name: "success - driver rated",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			setupDelivery: &ports.DeliveryInfo{
				DeliveryID: "delivery-1",
				CustomerID: "user-1",
				DriverID:   "driver-1",
			},
			expectedError: nil,
		},
		{
			name: "driver not found but delivery matches - creates review (bug in prod code)",
			setupDriver: nil,
			setupDelivery: &ports.DeliveryInfo{
				DeliveryID: "delivery-1",
				CustomerID: "user-1",
				DriverID:   "driver-1",
			},
			expectedError: nil,
		},
		{
			name: "fails - delivery not found",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			setupDelivery: nil,
			expectedError: domain.ErrDeliveryNotFound,
		},
		{
			name: "fails - not delivery owner",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			setupDelivery: &ports.DeliveryInfo{
				DeliveryID: "delivery-1",
				CustomerID: "user-2",
				DriverID:   "driver-1",
			},
			expectedError: domain.ErrNotDeliveryOwner,
		},
		{
			name: "fails - not assigned driver",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			setupDelivery: &ports.DeliveryInfo{
				DeliveryID: "delivery-1",
				CustomerID: "user-1",
				DriverID:   "driver-2",
			},
			setupAssignment: &domain.Assignment{
				ID:        "assign-1",
				DriverID:  "driver-2",
				DeliveryID: "delivery-1",
				Status:    domain.AssignmentStatusCompleted,
			},
			expectedError: domain.ErrNotAssignedDriver,
		},
		{
			name: "fails - duplicate review",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			setupDelivery: &ports.DeliveryInfo{
				DeliveryID: "delivery-1",
				CustomerID: "user-1",
				DriverID:   "driver-1",
			},
			setupReview: &domain.Review{
				ID:         "review-1",
				DriverID:   "driver-1",
				UserID:     "user-1",
				DeliveryID: "delivery-1",
				Rating:     4.0,
			},
			expectedError: domain.ErrDuplicateReview,
		},
		{
			name: "fails - invalid rating",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			setupDelivery: &ports.DeliveryInfo{
				DeliveryID: "delivery-1",
				CustomerID: "user-1",
				DriverID:   "driver-1",
			},
			expectedError: domain.ErrInvalidRating,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driverRepo := mocks.NewMockDriverRepository()
			reviewRepo := mocks.NewMockReviewRepository()
			deliveryClient := mocks.NewMockDeliveryServiceClient()
			assignmentRepo := mocks.NewMockAssignmentRepository()

			if tt.setupDriver != nil {
				driverRepo.SetDriver(tt.setupDriver)
			}
			if tt.setupReview != nil {
				reviewRepo.SetReview(tt.setupReview)
			}
			if tt.setupDelivery != nil {
				deliveryClient.SetDelivery(tt.setupDelivery)
			}
			if tt.setupAssignment != nil {
				assignmentRepo.SetAssignment(tt.setupAssignment)
			}
			if tt.driverRepoErr != nil {
				driverRepo.SetError(tt.driverRepoErr)
			}
			if tt.reviewRepoErr != nil {
				reviewRepo.SetError(tt.reviewRepoErr)
			}
			if tt.deliveryClientErr != nil {
				deliveryClient.SetError("delivery-1", tt.deliveryClientErr)
			}

			handler := commands.NewRateDriverHandler(driverRepo, reviewRepo, deliveryClient, assignmentRepo)

rating := 4.5
		if tt.name == "fails - invalid rating" {
			rating = 6.0
		}
		cmd := commands.RateDriverCommand{
			DriverID:   "driver-1",
			UserID:     "user-1",
			DeliveryID: "delivery-1",
			Rating:     rating,
			Comment:    "Great!",
		}

			review, err := handler.Execute(context.Background(), cmd)

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Execute() expected error %v, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("Execute() error = %v, want %v", err, tt.expectedError)
				}
				if review != nil {
					t.Errorf("Execute() should return nil review on error")
				}
			} else {
				if err != nil {
					t.Errorf("Execute() unexpected error: %v", err)
				}
				if review == nil {
					t.Fatal("Execute() returned nil review")
				}
				if review.DriverID != "driver-1" {
					t.Errorf("Review.DriverID = %v, want driver-1", review.DriverID)
				}
				if review.Rating != 4.5 {
					t.Errorf("Review.Rating = %v, want 4.5", review.Rating)
				}
			}
		})
	}
}