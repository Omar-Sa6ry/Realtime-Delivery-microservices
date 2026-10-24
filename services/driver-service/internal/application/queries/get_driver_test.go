package queries_test

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/queries"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

func TestGetDriverQuery_Execute(t *testing.T) {
	tests := []struct {
		name          string
		setupDriver   *domain.Driver
		setupError    error
		expectedError error
	}{
		{
			name: "success - driver found",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				UserID:    "user-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError: nil,
		},
		{
			name: "fails - driver not found",
			setupDriver: nil,
			expectedError: domain.ErrDriverNotFound,
		},
		{
			name: "fails - repo error",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				UserID:    "user-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			setupError: domain.ErrInternal,
			expectedError: domain.ErrInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driverRepo := mocks.NewMockDriverRepository()

			if tt.setupDriver != nil {
				driverRepo.SetDriver(tt.setupDriver)
			}
			if tt.setupError != nil {
				driverRepo.SetError(tt.setupError)
			}

			query := queries.NewGetDriverQuery("driver-1", driverRepo)

			driver, err := query.Execute(context.Background())

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Execute() expected error %v, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("Execute() error = %v, want %v", err, tt.expectedError)
				}
				if driver != nil {
					t.Errorf("Execute() should return nil driver on error")
				}
			} else {
				if err != nil {
					t.Errorf("Execute() unexpected error: %v", err)
				}
				if driver == nil {
					t.Fatal("Execute() returned nil driver")
				}
				if driver.ID != "driver-1" {
					t.Errorf("Driver.ID = %v, want driver-1", driver.ID)
				}
				if driver.UserID != "user-1" {
					t.Errorf("Driver.UserID = %v, want user-1", driver.UserID)
				}
			}
		})
	}
}