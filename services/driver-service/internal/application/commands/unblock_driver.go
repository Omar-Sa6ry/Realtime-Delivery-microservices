package commands

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// UnblockDriverCommand represents the input for unblocking a driver.
type UnblockDriverCommand struct {
	DriverID string
	Reason   string
}

// UnblockDriverHandler handles unblocking a driver.
type UnblockDriverHandler struct {
	driverRepo     ports.DriverRepository
	eventPublisher ports.EventPublisher
}

// NewUnblockDriverHandler creates a new UnblockDriverHandler.
func NewUnblockDriverHandler(
	driverRepo ports.DriverRepository,
	eventPublisher ports.EventPublisher,
) *UnblockDriverHandler {
	return &UnblockDriverHandler{
		driverRepo:     driverRepo,
		eventPublisher: eventPublisher,
	}
}

// Execute performs the unblock operation.
func (h *UnblockDriverHandler) Execute(ctx context.Context, cmd UnblockDriverCommand) (*domain.Driver, error) {
	driver, err := h.driverRepo.FindByID(ctx, cmd.DriverID)
	if err != nil {
		return nil, err
	}
	if driver == nil {
		return nil, domain.ErrDriverNotFound
	}

	if err := driver.Unblock(); err != nil {
		return nil, err
	}

	if err := h.driverRepo.Save(ctx, driver); err != nil {
		return nil, err
	}

	// We can publish an event here if needed, e.g., DriverUnblocked
	// h.eventPublisher.PublishDriverUnblocked(ctx, driver.ID, cmd.Reason)

	return driver, nil
}
