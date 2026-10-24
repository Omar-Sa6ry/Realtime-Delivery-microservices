package commands_test

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/commands"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

func TestAcceptAssignmentCommand_Execute(t *testing.T) {
	tests := []struct {
		name           string
		setupAssignment *domain.Assignment
		setupError     error
		driverID       string
		expectedError  error
		expectedStatus domain.AssignmentStatus
	}{
		{
			name: "success - driver accepts assignment",
			setupAssignment: &domain.Assignment{
				ID:        "assign-1",
				DriverID:  "driver-1",
				DeliveryID: "delivery-1",
				Status:    domain.AssignmentStatusOffered,
			},
			driverID: "driver-1",
			expectedError: nil,
			expectedStatus: domain.AssignmentStatusAccepted,
		},
		{
			name: "fails - assignment not found",
			setupAssignment: nil,
			driverID: "driver-1",
			expectedError: domain.ErrAssignmentNotFound,
			expectedStatus: domain.AssignmentStatusOffered,
		},
		{
			name: "fails - assignment belongs to different driver",
			setupAssignment: &domain.Assignment{
				ID:        "assign-1",
				DriverID:  "driver-2",
				DeliveryID: "delivery-1",
				Status:    domain.AssignmentStatusOffered,
			},
			driverID: "driver-1",
			expectedError: domain.ErrAssignmentInvalidState,
			expectedStatus: domain.AssignmentStatusOffered,
		},
		{
			name: "fails - assignment not in offered state",
			setupAssignment: &domain.Assignment{
				ID:        "assign-1",
				DriverID:  "driver-1",
				DeliveryID: "delivery-1",
				Status:    domain.AssignmentStatusAccepted,
			},
			driverID: "driver-1",
			expectedError: domain.ErrAssignmentInvalidState,
			expectedStatus: domain.AssignmentStatusAccepted,
		},
		{
			name: "fails - repo returns error",
			setupAssignment: &domain.Assignment{
				ID:        "assign-1",
				DriverID:  "driver-1",
				DeliveryID: "delivery-1",
				Status:    domain.AssignmentStatusOffered,
			},
			setupError: domain.ErrInternal,
			driverID: "driver-1",
			expectedError: domain.ErrInternal,
			expectedStatus: domain.AssignmentStatusAccepted, // Status changed in memory before save
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assignmentRepo := mocks.NewMockAssignmentRepository()
			eventPub := mocks.NewMockEventPublisher()

			if tt.setupAssignment != nil {
				assignmentRepo.SetAssignment(tt.setupAssignment)
			}
			if tt.setupError != nil {
				assignmentRepo.SetError(tt.setupError)
			}

			cmd := commands.NewAcceptAssignmentCommand("assign-1", tt.driverID, assignmentRepo, eventPub)

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

			// Verify assignment status in repo
			assignment, _ := assignmentRepo.FindByID(context.Background(), "assign-1")
			if assignment != nil && assignment.Status != tt.expectedStatus {
				t.Errorf("Assignment status in repo = %v, want %v", assignment.Status, tt.expectedStatus)
			}
		})
	}
}