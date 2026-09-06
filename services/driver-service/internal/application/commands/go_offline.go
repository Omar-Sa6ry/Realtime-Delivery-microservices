package commands

import (
	"context"
	"log"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// GoOfflineCommand transitions a driver from AVAILABLE to OFFLINE state.
type GoOfflineCommand struct {
	driverID   string
	driverRepo ports.DriverRepository
	eventPublisher ports.EventPublisher
}

// NewGoOfflineCommand creates a new GoOfflineCommand.
func NewGoOfflineCommand(driverID string, driverRepo ports.DriverRepository, eventPublisher ports.EventPublisher) *GoOfflineCommand {
	return &GoOfflineCommand{
		driverID:       driverID,
		driverRepo:     driverRepo,
		eventPublisher: eventPublisher,
	}
}

// Execute transitions the driver to offline state.
func (c *GoOfflineCommand) Execute(ctx context.Context) error {
	// Find driver by ID
	driver, err := c.driverRepo.FindByID(ctx, c.driverID)
	if err != nil {
		log.Printf("go_offline: failed to find driver %s: %v", c.driverID, err)
		return err
	}
	if driver == nil {
		return domain.ErrDriverNotFound
	}

	// Validate transition from AVAILABLE to OFFLINE
	if driver.Status != domain.DriverStatusAvailable {
		return domain.ErrDriverNotOffline
	}

	// Apply state transition
	driver.Status = domain.DriverStatusOffline
	driver.UpdatedAt = time.Now()

	err = c.driverRepo.Save(ctx, driver)
	if err != nil {
		log.Printf("go_offline: failed to save driver %s: %v", c.driverID, err)
		return err
	}

	log.Printf("go_offline: driver %s is now OFFLINE", c.driverID)
	return nil
}