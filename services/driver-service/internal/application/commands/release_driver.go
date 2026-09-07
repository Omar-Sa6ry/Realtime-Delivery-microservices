package commands

import (
	"context"
	"log"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// ReleaseDriverCommand releases a driver from their current assignment.
type ReleaseDriverCommand struct {
	driverID       string
	deliveryID     string
	driverRepo     ports.DriverRepository
	assignmentRepo ports.AssignmentRepository
	eventPublisher ports.EventPublisher
}

// NewReleaseDriverCommand creates a new ReleaseDriverCommand.
func NewReleaseDriverCommand(driverID, deliveryID string, driverRepo ports.DriverRepository,
	assignmentRepo ports.AssignmentRepository, eventPublisher ports.EventPublisher) *ReleaseDriverCommand {

	return &ReleaseDriverCommand{
		driverID:       driverID,
		deliveryID:     deliveryID,
		driverRepo:     driverRepo,
		assignmentRepo: assignmentRepo,
		eventPublisher: eventPublisher,
	}
}

// Execute releases the driver from their assignment.
func (c *ReleaseDriverCommand) Execute(ctx context.Context) error {
	// FindActiveByDriver now returns (*domain.Assignment, error)
	assignment, err := c.assignmentRepo.FindActiveByDriver(ctx, c.driverID)
	if err != nil || assignment == nil {
		log.Printf("release_driver: no active assignment found for driver %s", c.driverID)
		return domain.ErrAssignmentNotFound
	}

	// Update assignment status to COMPLETED
	if err = c.assignmentRepo.UpdateStatus(ctx, assignment.ID, string(domain.AssignmentStatusCompleted)); err != nil {
		log.Printf("release_driver: failed to update assignment %s status: %v", assignment.ID, err)
		return err
	}

	// Update driver state back to AVAILABLE
	driver, err := c.driverRepo.FindByID(ctx, c.driverID)
	if err != nil {
		log.Printf("release_driver: failed to find driver %s: %v", c.driverID, err)
		return err
	}
	if driver != nil {
		driver.Status = domain.DriverStatusAvailable
		driver.UpdatedAt = time.Now()
		if err = c.driverRepo.Save(ctx, driver); err != nil {
			log.Printf("release_driver: failed to restore driver %s state: %v", c.driverID, err)
			return err
		}
	}

	// Publish driver available event (best-effort)
	if err = c.eventPublisher.PublishDriverAvailable(ctx, c.driverID); err != nil {
		log.Printf("release_driver: failed to publish driver available event: %v", err)
	}

	log.Printf("release_driver: driver %s released from delivery %s", c.driverID, c.deliveryID)
	return nil
}