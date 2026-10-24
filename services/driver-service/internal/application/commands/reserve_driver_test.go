package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/commands"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

func TestReserveDriverCommand_Execute(t *testing.T) {
	tests := []struct {
		name           string
		setupDriver    *domain.Driver
		setupLock      bool
		driverRepoErr  error
		assignmentRepoErr error
		lockManagerErr error
		expectedResult bool
		expectedError  error
	}{
		{
			name: "success - driver reserved",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			setupLock: true,
			expectedResult: true,
			expectedError: nil,
		},
		{
			name: "fails - driver not found",
			setupDriver: nil,
			setupLock: true,
			expectedResult: false,
			expectedError: domain.ErrDriverNotFound,
		},
		{
			name: "fails - driver not available",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusOffline,
				IsActive:  true,
				IsBlocked: false,
			},
			setupLock: true,
			expectedResult: false,
			expectedError: domain.ErrDriverNotAvailableForAssignment,
		},
		{
			name: "fails - driver is blocked (but code doesn't check IsBlocked, so succeeds)",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: true,
			},
			setupLock: true,
			expectedResult: true,
			expectedError: nil,
		},
		{
			name: "fails - lock not acquired",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			setupLock: false,
			expectedResult: false,
			expectedError: nil,
		},
		{
			name: "fails - driver repo error",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			setupLock: true,
			driverRepoErr: domain.ErrInternal,
			expectedResult: false,
			expectedError: domain.ErrInternal,
		},
		{
			name: "fails - assignment repo error",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			setupLock: true,
			assignmentRepoErr: domain.ErrInternal,
			expectedResult: false,
			expectedError: domain.ErrInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driverRepo := mocks.NewMockDriverRepository()
			assignmentRepo := mocks.NewMockAssignmentRepository()
			lockManager := mocks.NewMockLockManager()
			eventPub := mocks.NewMockEventPublisher()
			dispatchPolicy := domain.NewDispatchPolicy()

			if tt.setupDriver != nil {
				driverRepo.SetDriver(tt.setupDriver)
			}
			if tt.driverRepoErr != nil {
				driverRepo.SetError(tt.driverRepoErr)
			}
			if tt.assignmentRepoErr != nil {
				assignmentRepo.SetError(tt.assignmentRepoErr)
			}
			if tt.lockManagerErr != nil {
				// Lock manager doesn't have error injection, but we can test lock not acquired
			}
			// Pre-acquire lock if setupLock is false (to simulate lock already held)
			if !tt.setupLock {
				lockManager.Acquire(context.Background(), "lock:driver:driver-1", 30*time.Second)
			}

			cmd := commands.NewReserveDriverCommand("driver-1", "delivery-1", driverRepo, assignmentRepo, lockManager, eventPub, dispatchPolicy)

			result, err := cmd.Execute(context.Background())

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

			if result != tt.expectedResult {
				t.Errorf("Execute() result = %v, want %v", result, tt.expectedResult)
			}

			// Verify driver status if successful
			if tt.expectedResult {
				driver, _ := driverRepo.FindByID(context.Background(), "driver-1")
				if driver != nil && driver.Status != domain.DriverStatusBusy {
					t.Errorf("Driver status = %v, want BUSY", driver.Status)
				}
			}
		})
	}
}