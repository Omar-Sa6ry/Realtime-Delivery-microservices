package commands_test

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/commands"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

func TestGoOnlineCommand_Execute(t *testing.T) {
	tests := []struct {
		name          string
		setupDriver   *domain.Driver
		setupError    error
		expectedError error
		expectedStatus domain.DriverStatus
	}{
		{
			name: "success - driver goes online",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusOffline,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError: nil,
			expectedStatus: domain.DriverStatusAvailable,
		},
		{
			name: "fails - driver not found",
			setupDriver: nil,
			expectedError: domain.ErrDriverNotFound,
			expectedStatus: domain.DriverStatusOffline,
		},
		{
			name: "fails - driver already online",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError: domain.ErrDriverAlreadyOnline,
			expectedStatus: domain.DriverStatusAvailable,
		},
		{
			name: "fails - driver is busy",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusBusy,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError: domain.ErrDriverAlreadyOnline,
			expectedStatus: domain.DriverStatusBusy,
		},
		{
			name: "fails - repo returns error",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusOffline,
				IsActive:  true,
				IsBlocked: false,
			},
			setupError: domain.ErrInternal,
			expectedError: domain.ErrInternal,
			expectedStatus: domain.DriverStatusAvailable, // Status changed in memory before save
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driverRepo := mocks.NewMockDriverRepository()
			eventPub := mocks.NewMockEventPublisher()

			if tt.setupDriver != nil {
				driverRepo.SetDriver(tt.setupDriver)
			}
			if tt.setupError != nil {
				driverRepo.SetError(tt.setupError)
			}

			cmd := commands.NewGoOnlineCommand("driver-1", driverRepo, eventPub)

			err := cmd.Execute(context.Background())

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Execute() expected error %v, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("Execute() error = %v, want %v", err, tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("Execute() unexpected error: %v", err)
				}
			}

			// Verify driver status in repo
			driver, _ := driverRepo.FindByID(context.Background(), "driver-1")
			if driver != nil && driver.Status != tt.expectedStatus {
				t.Errorf("Driver status in repo = %v, want %v", driver.Status, tt.expectedStatus)
			}
		})
	}
}