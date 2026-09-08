package commands

import (
	"context"
	"log"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc/proto"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
	"github.com/google/uuid"
)

// RegisterDriverCommand represents the input for registering a driver.
type RegisterDriverCommand struct {
	UserID       string
	VehicleType  domain.VehicleType
	PlateNumber  string
	CapacityKg   int64
	Capabilities []string
	ServiceArea  string
}

// RegisterDriverHandler handles driver registration.
type RegisterDriverHandler struct {
	driverRepo     ports.DriverRepository
	eventPublisher ports.EventPublisher
	userClient     proto.UserServiceClient
}

// NewRegisterDriverHandler creates a new RegisterDriverHandler.
func NewRegisterDriverHandler(
	driverRepo ports.DriverRepository,
	eventPublisher ports.EventPublisher,
	userClient proto.UserServiceClient,
) *RegisterDriverHandler {
	return &RegisterDriverHandler{
		driverRepo:     driverRepo,
		eventPublisher: eventPublisher,
		userClient:     userClient,
	}
}

// Execute performs the registration.
func (h *RegisterDriverHandler) Execute(ctx context.Context, cmd RegisterDriverCommand) (*domain.Driver, error) {
	// 1. Update user role to DRIVER via User Service gRPC
	if h.userClient != nil {
		req := &proto.UpdateUserRoleRequest{
			UserId: cmd.UserID,
			Role:   "DRIVER",
		}
		resp, err := h.userClient.UpdateUserRole(ctx, req)
		if err != nil || !resp.Success {
			log.Printf("failed to update user role to DRIVER for user %s: %v", cmd.UserID, err)
			return nil, domain.ErrInternal
		}
	} else {
		log.Printf("userClient is nil, skipping user role update for user %s", cmd.UserID)
	}

	// 2. Create the driver in domain
	driver := &domain.Driver{
		ID:           uuid.New().String(),
		UserID:       cmd.UserID,
		Status:       domain.DriverStatusOffline, // Starts offline
		Capabilities: cmd.Capabilities,
		ServiceArea:  cmd.ServiceArea,
		Vehicle: domain.VehicleInfo{
			Type:        cmd.VehicleType,
			PlateNumber: cmd.PlateNumber,
			CapacityKg:  cmd.CapacityKg,
		},
	}

	// 3. Save to repository
	if err := h.driverRepo.Save(ctx, driver); err != nil {
		return nil, err
	}

	// 4. Publish Event
	if err := h.eventPublisher.PublishDriverCreated(ctx, driver.ID, driver.UserID); err != nil {
		log.Printf("failed to publish driver created event: %v", err)
	}

	return driver, nil
}
