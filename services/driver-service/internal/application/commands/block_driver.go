package commands

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// BlockDriverCommand represents the input for blocking a driver.
type BlockDriverCommand struct {
	DriverID string
	Reason   string
}

// BlockDriverHandler handles blocking a driver.
type BlockDriverHandler struct {
	driverRepo     ports.DriverRepository
	eventPublisher ports.EventPublisher
}

// NewBlockDriverHandler creates a new BlockDriverHandler.
func NewBlockDriverHandler(
	driverRepo ports.DriverRepository,
	eventPublisher ports.EventPublisher,
) *BlockDriverHandler {
	return &BlockDriverHandler{
		driverRepo:     driverRepo,
		eventPublisher: eventPublisher,
	}
}

// Execute performs the block operation.
func (h *BlockDriverHandler) Execute(ctx context.Context, cmd BlockDriverCommand) (*domain.Driver, error) {
	driver, err := h.driverRepo.FindByID(ctx, cmd.DriverID)
	if err != nil {
		return nil, err
	}
	if driver == nil {
		return nil, domain.ErrDriverNotFound
	}

	if err := driver.Block(); err != nil {
		return nil, err
	}

	if err := h.driverRepo.Save(ctx, driver); err != nil {
		return nil, err
	}

	// We can publish an event here if needed, e.g., DriverBlocked
	// h.eventPublisher.PublishDriverBlocked(ctx, driver.ID, cmd.Reason)

	return driver, nil
}
