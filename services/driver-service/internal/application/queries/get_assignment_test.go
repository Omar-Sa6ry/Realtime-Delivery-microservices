package queries_test

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/queries"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

func TestGetAssignmentQuery_Execute(t *testing.T) {
	tests := []struct {
		name            string
		setupAssignment *domain.Assignment
		setupError      error
		expectedError   error
	}{
		{
			name: "success - assignment found",
			setupAssignment: &domain.Assignment{
				ID:        "assign-1",
				DriverID:  "driver-1",
				DeliveryID: "delivery-1",
				Status:    domain.AssignmentStatusOffered,
			},
			expectedError: nil,
		},
		{
			name: "fails - assignment not found",
			setupAssignment: nil,
			expectedError: domain.ErrAssignmentNotFound,
		},
		{
			name: "fails - repo error",
			setupAssignment: &domain.Assignment{
				ID:        "assign-1",
				DriverID:  "driver-1",
				DeliveryID: "delivery-1",
				Status:    domain.AssignmentStatusOffered,
			},
			setupError: domain.ErrInternal,
			expectedError: domain.ErrInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assignmentRepo := mocks.NewMockAssignmentRepository()

			if tt.setupAssignment != nil {
				assignmentRepo.SetAssignment(tt.setupAssignment)
			}
			if tt.setupError != nil {
				assignmentRepo.SetError(tt.setupError)
			}

			query := queries.NewGetAssignmentQuery("assign-1", assignmentRepo)

			assignment, err := query.Execute(context.Background())

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Execute() expected error %v, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("Execute() error = %v, want %v", err, tt.expectedError)
				}
				if assignment != nil {
					t.Errorf("Execute() should return nil assignment on error")
				}
			} else {
				if err != nil {
					t.Errorf("Execute() unexpected error: %v", err)
				}
				if assignment == nil {
					t.Fatal("Execute() returned nil assignment")
				}
				if assignment.ID != "assign-1" {
					t.Errorf("Assignment.ID = %v, want assign-1", assignment.ID)
				}
			}
		})
	}
}