package commands

import (
	"context"
	"log"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// AcceptAssignmentCommand accepts a driver's assignment offer.
type AcceptAssignmentCommand struct {
	assignmentID string
	driverID     string
	assignmentRepo ports.AssignmentRepository
	eventPublisher ports.EventPublisher
}

// NewAcceptAssignmentCommand creates a new AcceptAssignmentCommand.
func NewAcceptAssignmentCommand(assignmentID, driverID string, assignmentRepo ports.AssignmentRepository,
	eventPublisher ports.EventPublisher) *AcceptAssignmentCommand {

	return &AcceptAssignmentCommand{
		assignmentID: assignmentID,
		driverID:     driverID,
		assignmentRepo: assignmentRepo,
		eventPublisher: eventPublisher,
	}
}

// Execute accepts the assignment offer.
func (c *AcceptAssignmentCommand) Execute(ctx context.Context) error {
	// Find assignment by ID
	assignment, err := c.assignmentRepo.FindByID(ctx, c.assignmentID)
	if err != nil {
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

	// Transition assignment from OFFERED to ACCEPTED
	now := time.Now()
	assignment.Status = domain.AssignmentStatusAccepted
	assignment.AcceptedAt = &now
	assignment.UpdatedAt = time.Now()

	err = c.assignmentRepo.Save(ctx, assignment)
	if err != nil {
		return err
	}

	// Publish assignment accepted event
	err = c.eventPublisher.PublishAssignmentAccepted(ctx, assignment.ID, assignment.DeliveryID, assignment.DriverID)
	if err != nil {
		log.Printf("accept_assignment: failed to publish assignment accepted event: %v", err)
	}

	log.Printf("accept_assignment: assignment %s accepted by driver %s", c.assignmentID, c.driverID)
	return nil
}