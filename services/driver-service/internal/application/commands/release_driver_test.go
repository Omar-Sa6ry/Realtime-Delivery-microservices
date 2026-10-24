package commands_test

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/commands"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

func TestReleaseDriverCommand_Execute(t *testing.T) {
	tests := []struct {
		name            string
		setupDriver     *domain.Driver
		setupAssignment *domain.Assignment
		driverRepoErr   error
		assignmentRepoErr error
		expectedError   error
	}{
		{
			name: "success - driver released",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusBusy,
				IsActive:  true,
				IsBlocked: false,
			},
			setupAssignment: &domain.Assignment{
				ID:        "assign-1",
				DriverID:  "driver-1",
				DeliveryID: "delivery-1",
				Status:    domain.AssignmentStatusActive,
			},
			expectedError: nil,
		},
		{
			name: "fails - no active assignment",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusBusy,
				IsActive:  true,
				IsBlocked: false,
			},
			setupAssignment: nil,
			expectedError: domain.ErrAssignmentNotFound,
		},
		{
			name: "fails - driver not found",
			setupDriver: nil,
			setupAssignment: &domain.Assignment{
				ID:        "assign-1",
				DriverID:  "driver-1",
				DeliveryID: "delivery-1",
				Status:    domain.AssignmentStatusActive,
			},
			expectedError: nil, // Command doesn't fail if driver not found, just skips driver update
		},
		{
			name: "fails - assignment repo update status error",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusBusy,
				IsActive:  true,
				IsBlocked: false,
			},
			setupAssignment: &domain.Assignment{
				ID:        "assign-1",
				DriverID:  "driver-1",
				DeliveryID: "delivery-1",
				Status:    domain.AssignmentStatusActive,
			},
			assignmentRepoErr: domain.ErrInternal,
			expectedError: domain.ErrInternal,
		},
		{
			name: "fails - driver repo save error",
			setupDriver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusBusy,
				IsActive:  true,
				IsBlocked: false,
			},
			setupAssignment: &domain.Assignment{
				ID:        "assign-1",
				DriverID:  "driver-1",
				DeliveryID: "delivery-1",
				Status:    domain.AssignmentStatusActive,
			},
			driverRepoErr: domain.ErrInternal,
			expectedError: domain.ErrInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driverRepo := mocks.NewMockDriverRepository()
			assignmentRepo := mocks.NewMockAssignmentRepository()
			eventPub := mocks.NewMockEventPublisher()

			if tt.setupDriver != nil {
				driverRepo.SetDriver(tt.setupDriver)
			}
			if tt.setupAssignment != nil {
				assignmentRepo.SetAssignment(tt.setupAssignment)
			}
			if tt.driverRepoErr != nil {
				driverRepo.SetError(tt.driverRepoErr)
			}
			if tt.assignmentRepoErr != nil {
				assignmentRepo.SetError(tt.assignmentRepoErr)
			}

			cmd := commands.NewReleaseDriverCommand("driver-1", "delivery-1", driverRepo, assignmentRepo, eventPub)

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

			// Verify assignment status if successful
			if tt.expectedError == nil && tt.setupAssignment != nil {
				assignment, _ := assignmentRepo.FindByID(context.Background(), "assign-1")
				if assignment != nil && assignment.Status != domain.AssignmentStatusCompleted {
					t.Errorf("Assignment status = %v, want COMPLETED", assignment.Status)
				}
			}

			// Verify driver status if successful and driver exists
			if tt.expectedError == nil && tt.setupDriver != nil {
				driver, _ := driverRepo.FindByID(context.Background(), "driver-1")
				if driver != nil && driver.Status != domain.DriverStatusAvailable {
					t.Errorf("Driver status = %v, want AVAILABLE", driver.Status)
				}
			}
		})
	}
}