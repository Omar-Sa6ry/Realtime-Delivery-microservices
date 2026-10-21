package commands

import (
	"context"
	"log"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

type RejectAssignmentCommand struct {
	assignmentID string
	driverID     string
	reason       string
	assignmentRepo ports.AssignmentRepository
	eventPublisher ports.EventPublisher
}

func NewRejectAssignmentCommand(assignmentID, driverID, reason string, assignmentRepo ports.AssignmentRepository,
	eventPublisher ports.EventPublisher) *RejectAssignmentCommand {

	return &RejectAssignmentCommand{
		assignmentID: assignmentID,
		driverID:     driverID,
		reason:       reason,
		assignmentRepo: assignmentRepo,
		eventPublisher: eventPublisher,
	}
}

func (c *RejectAssignmentCommand) Execute(ctx context.Context) error {
	// Find assignment by ID
	assignment, err := c.assignmentRepo.FindByID(ctx, c.assignmentID)
	if err != nil {
		log.Printf("reject_assignment: failed to find assignment %s: %v", c.assignmentID, err)
		return err
	}
	if assignment == nil {
		return domain.ErrAssignmentNotFound
	}

	// Validate assignment belongs to this driver
	if assignment.DriverID != c.driverID {
		return domain.ErrAssignmentInvalidState
	}

	// Validate assignment is in OFFERED state
	if assignment.Status != domain.AssignmentStatusOffered {
		return domain.ErrAssignmentInvalidState
	}

	// Transition assignment from OFFERED to REJECTED
	now := time.Now()
	assignment.Status = domain.AssignmentStatusRejected
	assignment.RejectedAt = &now
	assignment.UpdatedAt = time.Now()

	err = c.assignmentRepo.Save(ctx, assignment)
	if err != nil {
		log.Printf("reject_assignment: failed to save assignment %s: %v", c.assignmentID, err)
		return err
	}

	// Publish assignment rejected event
	err = c.eventPublisher.PublishAssignmentRejected(ctx, assignment.ID, assignment.DeliveryID, c.driverID, c.reason)
	if err != nil {
		log.Printf("reject_assignment: failed to publish assignment rejected event: %v", err)
	}

	log.Printf("reject_assignment: assignment %s rejected by driver %s", c.assignmentID, c.driverID)
	return nil
}