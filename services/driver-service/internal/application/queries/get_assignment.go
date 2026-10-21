package queries

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

type GetAssignmentQuery struct {
	AssignmentID string
	assignmentRepo ports.AssignmentRepository
}

func NewGetAssignmentQuery(assignmentID string, assignmentRepo ports.AssignmentRepository) *GetAssignmentQuery {
	return &GetAssignmentQuery{
		AssignmentID: assignmentID,
		assignmentRepo: assignmentRepo,
	}
}

func (q *GetAssignmentQuery) Execute(ctx context.Context) (*domain.Assignment, error) {
	assignment, err := q.assignmentRepo.FindByID(ctx, q.AssignmentID)
	if err != nil {
		return nil, err
	}
	if assignment == nil {
		return nil, domain.ErrAssignmentNotFound
	}
	return assignment, nil
}