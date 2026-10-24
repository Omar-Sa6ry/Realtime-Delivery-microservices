package commands_test

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/commands"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

func TestUnblockDriverHandler_Execute(t *testing.T) {
	tests := []struct {
		name          string
		setupDriver   *domain.Driver
		setupError    error
		expectedError error
		expectedBlocked bool
	}{
		{
			name: "success - driver unblocked",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: true,
			},
			expectedError: nil,
			expectedBlocked: false,
		},
		{
			name: "fails - driver not found",
			setupDriver: nil,
			expectedError: domain.ErrDriverNotFound,
			expectedBlocked: false,
		},
		{
			name: "fails - repo error",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: true,
			},
			setupError: domain.ErrInternal,
			expectedError: domain.ErrInternal,
			expectedBlocked: true,
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

			handler := commands.NewUnblockDriverHandler(driverRepo, eventPub)

			driver, err := handler.Execute(context.Background(), commands.UnblockDriverCommand{DriverID: "driver-1"})

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
				if driver.IsBlocked != tt.expectedBlocked {
					t.Errorf("Driver.IsBlocked = %v, want %v", driver.IsBlocked, tt.expectedBlocked)
				}
			}
		})
	}
}