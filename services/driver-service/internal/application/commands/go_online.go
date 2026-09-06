package commands

import (
	"context"
	"log"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// GoOnlineCommand transitions a driver from OFFLINE to AVAILABLE state.
type GoOnlineCommand struct {
	driverID   string
	driverRepo ports.DriverRepository
	eventPublisher ports.EventPublisher
}

// NewGoOnlineCommand creates a new GoOnlineCommand.
func NewGoOnlineCommand(driverID string, driverRepo ports.DriverRepository, eventPublisher ports.EventPublisher) *GoOnlineCommand {
	return &GoOnlineCommand{
		driverID:       driverID,
		driverRepo:     driverRepo,
		eventPublisher: eventPublisher,
	}
}

// Execute transitions the driver to online state.
func (c *GoOnlineCommand) Execute(ctx context.Context) error {
	// Find driver by ID
	driver, err := c.driverRepo.FindByID(ctx, c.driverID)
	if err != nil {
		log.Printf("go_online: failed to find driver %s: %v", c.driverID, err)
		return err
	}
	if driver == nil {
		return domain.ErrDriverNotFound
	}

	// Validate transition from OFFLINE to AVAILABLE
	if driver.Status != domain.DriverStatusOffline {
		return domain.ErrDriverAlreadyOnline
	}

	// Apply state transition
	driver.Status = domain.DriverStatusAvailable
	driver.UpdatedAt = time.Now()

	err = c.driverRepo.Save(ctx, driver)
	if err != nil {
		log.Printf("go_online: failed to save driver %s: %v", c.driverID, err)
		return err
	}

	// Publish driver available event
	err = c.eventPublisher.PublishDriverAvailable(ctx, c.driverID)
	if err != nil {
		log.Printf("go_online: failed to publish driver available event: %v", err)
		// Don't fail the command - event is best-effort
	}

	log.Printf("go_online: driver %s is now AVAILABLE", c.driverID)
	return nil
}