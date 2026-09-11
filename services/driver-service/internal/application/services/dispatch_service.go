package services

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

type DispatchService struct {
	driverRepo     ports.DriverRepository
	assignmentRepo ports.AssignmentRepository
	locationStore  ports.LocationStore
	lockManager    ports.LockManager
	eventPublisher ports.EventPublisher
	dispatchPolicy *domain.DispatchPolicy
	mu             sync.Mutex
}

// NewDispatchService creates a new DispatchService.
func NewDispatchService(
	driverRepo ports.DriverRepository,
	assignmentRepo ports.AssignmentRepository,
	locationStore ports.LocationStore,
	lockManager ports.LockManager,
	eventPublisher ports.EventPublisher,
	dispatchPolicy *domain.DispatchPolicy,
) *DispatchService {
	return &DispatchService{
		driverRepo:     driverRepo,
		assignmentRepo: assignmentRepo,
		locationStore:  locationStore,
		lockManager:    lockManager,
		eventPublisher: eventPublisher,
		dispatchPolicy: dispatchPolicy,
	}
}

// FindAvailableDrivers finds drivers available near the given coordinates.
func (s *DispatchService) FindAvailableDrivers(ctx context.Context, lat, lng, radiusKm float64, vehicleType domain.VehicleType, deliveryID string) ([]domain.Candidate, error) {
	drivers, err := s.driverRepo.FindAvailableByLocation(ctx, lat, lng, radiusKm, vehicleType)
	if err != nil {
		log.Printf("dispatch service: find available by location failed: %v", err)
		return nil, err
	}

	var candidates []domain.Candidate
	for _, d := range drivers {
		candidates = append(candidates, domain.Candidate{
			DriverID:       d.ID,
			DistanceMeters: 0, // TODO: calculate from Redis GEOSEARCH result
			VehicleType:    d.Vehicle.Type,
			Status:         d.Status,
		})
	}

	return domain.NewDispatchPolicy().RankCandidates(candidates), nil
}

// ReserveDriver reserves a driver for a delivery using distributed locking and conditional state transition.
func (s *DispatchService) ReserveDriver(ctx context.Context, driverID, deliveryID string) (bool, error) {
	acquired, err := s.lockManager.Acquire(ctx, driverID, 30*time.Second)
	if err != nil {
		log.Printf("dispatch service: failed to acquire lock for driver %s: %v", driverID, err)
		return false, err
	}
	if !acquired {
		log.Printf("dispatch service: lock already held for driver %s", driverID)
		return false, domain.ErrDriverAlreadyReserved
	}
	defer s.lockManager.Release(ctx, driverID, "")

	driver, err := s.driverRepo.FindByID(ctx, driverID)
	if err != nil {
		return false, err
	}
	if driver == nil {
		return false, domain.ErrDriverNotFound
	}
	if driver.Status != domain.DriverStatusAvailable {
		return false, domain.ErrDriverNotAvailableForAssignment
	}

	// Transition to BUSY
	if err := driver.Reserve(); err != nil {
		return false, err
	}
	if err = s.driverRepo.Save(ctx, driver); err != nil {
		log.Printf("dispatch service: failed to save driver %s state: %v", driverID, err)
		return false, err
	}

	// Create assignment record as OFFERED
	now := time.Now()
	assignment := &domain.Assignment{
		ID:            deliveryID + "-" + driverID,
		DriverID:      driverID,
		DeliveryID:    deliveryID,
		Status:        domain.AssignmentStatusOffered,
		AttemptNumber: 1,
		OfferedAt:     now,
		ExpiresAt:     now.Add(10 * time.Minute),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err = s.assignmentRepo.Save(ctx, assignment); err != nil {
		log.Printf("dispatch service: failed to save assignment %s: %v", assignment.ID, err)
		// Rollback driver state
		driver.Status = domain.DriverStatusAvailable
		driver.UpdatedAt = time.Now()
		_ = s.driverRepo.Save(ctx, driver)
		return false, err
	}

	// Publish assignment offered event (best-effort)
	if err = s.eventPublisher.PublishAssignmentOffered(ctx, assignment.ID, assignment.DeliveryID, assignment.DriverID); err != nil {
		log.Printf("dispatch service: failed to publish assignment offered event: %v", err)
	}

	log.Printf("dispatch service: driver %s reserved for delivery %s", driverID, deliveryID)
	return true, nil
}

// ReleaseDriver releases a driver from their current assignment.
func (s *DispatchService) ReleaseDriver(ctx context.Context, driverID, deliveryID string) error {
	assignment, err := s.assignmentRepo.FindActiveByDriver(ctx, driverID)
	if err != nil || assignment == nil {
		return domain.ErrAssignmentNotFound
	}

	if err = s.assignmentRepo.UpdateStatus(ctx, assignment.ID, string(domain.AssignmentStatusCompleted)); err != nil {
		log.Printf("dispatch service: failed to update assignment %s status: %v", assignment.ID, err)
		return err
	}

	driver, err := s.driverRepo.FindByID(ctx, driverID)
	if err != nil {
		return err
	}
	if driver != nil {
		driver.Status = domain.DriverStatusAvailable
		driver.UpdatedAt = time.Now()
		if err = s.driverRepo.Save(ctx, driver); err != nil {
			log.Printf("dispatch service: failed to restore driver %s state: %v", driverID, err)
			return err
		}
	}

	if err = s.eventPublisher.PublishDriverAvailable(ctx, driverID); err != nil {
		log.Printf("dispatch service: failed to publish driver available event: %v", err)
	}

	log.Printf("dispatch service: driver %s released from delivery %s", driverID, deliveryID)
	return nil
}

// AcceptAssignment accepts a driver's assignment offer.
func (s *DispatchService) AcceptAssignment(ctx context.Context, assignmentID, driverID string) error {
	assignment, err := s.assignmentRepo.FindByID(ctx, assignmentID)
	if err != nil {
		return err
	}
	if assignment == nil {
		return domain.ErrAssignmentNotFound
	}
	if assignment.DriverID != driverID {
		return domain.ErrAssignmentInvalidState
	}
	if assignment.Status != domain.AssignmentStatusOffered {
		if assignment.Status == domain.AssignmentStatusAccepted {
			return nil // Idempotent
		}
		return domain.ErrAssignmentInvalidState
	}

	now := time.Now()
	assignment.Status = domain.AssignmentStatusAccepted
	assignment.AcceptedAt = &now
	assignment.UpdatedAt = now

	if err = s.assignmentRepo.Save(ctx, assignment); err != nil {
		return err
	}

	if err = s.eventPublisher.PublishAssignmentAccepted(ctx, assignment.ID, assignment.DeliveryID, assignment.DriverID); err != nil {
		log.Printf("dispatch service: failed to publish assignment accepted event: %v", err)
	}

	log.Printf("dispatch service: assignment %s accepted by driver %s", assignmentID, driverID)
	return nil
}

// RejectAssignment rejects a driver's assignment offer.
func (s *DispatchService) RejectAssignment(ctx context.Context, assignmentID, driverID, reason string) error {
	assignment, err := s.assignmentRepo.FindByID(ctx, assignmentID)
	if err != nil {
		return err
	}
	if assignment == nil {
		return domain.ErrAssignmentNotFound
	}
	if assignment.DriverID != driverID {
		return domain.ErrAssignmentInvalidState
	}
	if assignment.Status != domain.AssignmentStatusOffered {
		if assignment.Status == domain.AssignmentStatusRejected {
			return nil // Idempotent
		}
		return domain.ErrAssignmentInvalidState
	}

	now := time.Now()
	assignment.Status = domain.AssignmentStatusRejected
	assignment.RejectedAt = &now
	assignment.UpdatedAt = now

	if err = s.assignmentRepo.Save(ctx, assignment); err != nil {
		return err
	}

	// Release driver back to AVAILABLE
	driver, err := s.driverRepo.FindByID(ctx, driverID)
	if err == nil && driver != nil {
		driver.Status = domain.DriverStatusAvailable
		driver.UpdatedAt = time.Now()
		_ = s.driverRepo.Save(ctx, driver)
	}

	if err = s.eventPublisher.PublishAssignmentRejected(ctx, assignment.ID, assignment.DeliveryID, driverID, reason); err != nil {
		log.Printf("dispatch service: failed to publish assignment rejected event: %v", err)
	}

	log.Printf("dispatch service: assignment %s rejected by driver %s", assignmentID, driverID)
	return nil
}

// ReleaseDriverByAssignment releases a driver by assignment ID.
func (s *DispatchService) ReleaseDriverByAssignment(ctx context.Context, assignmentID string) error {
	assignment, err := s.assignmentRepo.FindByID(ctx, assignmentID)
	if err != nil {
		return err
	}
	if assignment == nil {
		return domain.ErrAssignmentNotFound
	}

	var newStatus domain.AssignmentStatus
	switch assignment.Status {
	case domain.AssignmentStatusActive, domain.AssignmentStatusAccepted:
		newStatus = domain.AssignmentStatusCompleted
	case domain.AssignmentStatusOffered:
		newStatus = domain.AssignmentStatusExpired
	default:
		// Already in terminal state — idempotent
		return nil
	}

	assignment.Status = newStatus
	assignment.UpdatedAt = time.Now()
	if err = s.assignmentRepo.Save(ctx, assignment); err != nil {
		return err
	}

	// Release driver
	driver, err := s.driverRepo.FindByID(ctx, assignment.DriverID)
	if err == nil && driver != nil {
		driver.Status = domain.DriverStatusAvailable
		driver.UpdatedAt = time.Now()
		if err = s.driverRepo.Save(ctx, driver); err != nil {
			return err
		}
		if err = s.eventPublisher.PublishDriverAvailable(ctx, assignment.DriverID); err != nil {
			log.Printf("dispatch service: failed to publish driver available event: %v", err)
		}
	}

	log.Printf("dispatch service: assignment %s released", assignmentID)
	return nil
}

// ValidateDriverID validates a driver ID using the driver repository.
func (s *DispatchService) ValidateDriverID(ctx context.Context, driverID string) (bool, string) {
	driver, err := s.driverRepo.FindByID(ctx, driverID)
	if err != nil || driver == nil {
		return false, ""
	}
	return true, string(driver.Status)
}

// ValidateUserID validates that a user ID has a linked driver profile.
func (s *DispatchService) ValidateUserID(ctx context.Context, userID string) (bool, string) {
	driver, err := s.driverRepo.FindByUserID(ctx, userID)
	if err != nil || driver == nil {
		return false, ""
	}
	return true, string(driver.Status)
}