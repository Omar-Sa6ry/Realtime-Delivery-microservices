package commands

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

type UnblockDriverCommand struct {
	DriverID string
	Reason   string
}

type UnblockDriverHandler struct {
	driverRepo     ports.DriverRepository
	eventPublisher ports.EventPublisher
}

func NewUnblockDriverHandler(
	driverRepo ports.DriverRepository,
	eventPublisher ports.EventPublisher,
) *UnblockDriverHandler {
	return &UnblockDriverHandler{
		driverRepo:     driverRepo,
		eventPublisher: eventPublisher,
	}
}

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

	return driver, nil
}
