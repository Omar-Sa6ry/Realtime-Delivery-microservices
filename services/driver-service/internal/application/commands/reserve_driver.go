package commands

import (
	"context"
	"log"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// ReserveDriverCommand reserves a driver for a delivery assignment.
type ReserveDriverCommand struct {
	driverID     string
	deliveryID   string
	driverRepo   ports.DriverRepository
	assignmentRepo ports.AssignmentRepository
	lockManager    ports.LockManager
	eventPublisher ports.EventPublisher
	dispatchPolicy *domain.DispatchPolicy
}

// NewReserveDriverCommand creates a new ReserveDriverCommand.
func NewReserveDriverCommand(driverID, deliveryID string, driverRepo ports.DriverRepository,
	assignmentRepo ports.AssignmentRepository, lockManager ports.LockManager,
	eventPublisher ports.EventPublisher, dispatchPolicy *domain.DispatchPolicy) *ReserveDriverCommand {

	return &ReserveDriverCommand{
		driverID:       driverID,
		deliveryID:     deliveryID,
		driverRepo:     driverRepo,
		assignmentRepo: assignmentRepo,
		lockManager:    lockManager,
		eventPublisher: eventPublisher,
		dispatchPolicy: dispatchPolicy,
	}
}

// Execute reserves the driver for the delivery.
func (c *ReserveDriverCommand) Execute(ctx context.Context) (bool, error) {
	// Acquire distributed lock for the driver
	acquired, err := c.lockManager.Acquire(ctx, "lock:driver:"+c.driverID, 30*time.Second)
	if err != nil {
		log.Printf("reserve_driver: failed to acquire lock for driver %s: %v", c.driverID, err)
		return false, err
	}
	if !acquired {
		log.Printf("reserve_driver: lock already held for driver %s", c.driverID)
		return false, nil
	}
	defer c.lockManager.Release(ctx, "lock:driver:"+c.driverID, "temp-token")

	// Check driver state from MongoDB
	driver, err := c.driverRepo.FindByID(ctx, c.driverID)
	if err != nil {
		log.Printf("reserve_driver: failed to find driver %s: %v", c.driverID, err)
		return false, err
	}
	if driver == nil {
		log.Printf("reserve_driver: driver %s not found", c.driverID)
		return false, domain.ErrDriverNotFound
	}

	// Check driver is available
	if driver.Status != domain.DriverStatusAvailable {
		log.Printf("reserve_driver: driver %s is not available, status: %s", c.driverID, driver.Status)
		return false, domain.ErrDriverNotAvailableForAssignment
	}

	// Conditionally update driver state from AVAILABLE to BUSY
	driver.Status = domain.DriverStatusBusy
	driver.UpdatedAt = time.Now()

	err = c.driverRepo.Save(ctx, driver)
	if err != nil {
		log.Printf("reserve_driver: failed to update driver %s state: %v", c.driverID, err)
		return false, err
	}

	// Create assignment record as OFFERED
	assignment := domain.Assignment{
		ID:          c.deliveryID + "-" + c.driverID,
		DriverID:    c.driverID,
		DeliveryID:  c.deliveryID,
		Status:      domain.AssignmentStatusOffered,
		AttemptNumber: 1,
		OfferedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(20 * time.Second),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = c.assignmentRepo.Save(ctx, assignment.ID, assignment.DriverID, assignment.DeliveryID, string(assignment.Status))
	if err != nil {
		log.Printf("reserve_driver: failed to save assignment %s: %v", assignment.ID, err)
		// Reset driver state on failure
		driver.Status = domain.DriverStatusAvailable
		driver.UpdatedAt = time.Now()
		c.driverRepo.Save(ctx, driver)
		return false, err
	}

	// Publish assignment offered event
	err = c.eventPublisher.PublishAssignmentOffered(ctx, assignment.ID, assignment.DeliveryID, assignment.DriverID)
	if err != nil {
		log.Printf("reserve_driver: failed to publish assignment offered event: %v", err)
		// Don't fail the whole operation - event publishing is best-effort
	}

	log.Printf("reserve_driver: driver %s reserved for delivery %s", c.driverID, c.deliveryID)
	return true, nil
}