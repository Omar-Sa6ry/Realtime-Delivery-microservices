package services

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// DispatchService orchestrates driver dispatch operations including finding available drivers,
// reserving drivers, and managing assignment state transitions.
type DispatchService struct {
	driverRepo      ports.DriverRepository
	assignmentRepo  ports.AssignmentRepository
	locationStore   ports.LocationStore
	lockManager     ports.LockManager
	eventPublisher  ports.EventPublisher
	dispatchPolicy  *domain.DispatchPolicy
	mu              sync.Mutex
}

// NewDispatchService creates a new DispatchService.
func NewDispatchService(driverRepo ports.DriverRepository, assignmentRepo ports.AssignmentRepository,
	locationStore ports.LocationStore, lockManager ports.LockManager, eventPublisher ports.EventPublisher,
	dispatchPolicy *domain.DispatchPolicy) *DispatchService {
	return &DispatchService{
		driverRepo:      driverRepo,
		assignmentRepo:  assignmentRepo,
		locationStore:   locationStore,
		lockManager:     lockManager,
		eventPublisher:  eventPublisher,
		dispatchPolicy:  dispatchPolicy,
	}
}

// FindAvailableDrivers finds drivers available near the given coordinates.
func (s *DispatchService) FindAvailableDrivers(ctx context.Context, lat, lng, radiusKm float64, vehicleType string, deliveryID string) ([]domain.Candidate, error) {
	// Search for drivers using Redis GEO
	// Filter by AVAILABLE state from MongoDB
	// Rank candidates by distance and compatibility
	// Return ranked candidates

	candidates, err := s.driverRepo.FindAvailableByLocation(ctx, lat, lng, radiusKm, vehicleType)
	if err != nil {
		log.Printf("dispatch service: find available by location failed: %v", err)
		return nil, err
	}

	// Convert to candidates and rank
	var candidatesSlice []domain.Candidate
	for _, d := range candidates {
		candidatesSlice = append(candidatesSlice, domain.Candidate{
			DriverID:       d.ID,
			DistanceMeters: 0, // would be calculated from GEOSEARCH
			VehicleType:    d.Vehicle.Type,
			Status:         d.Status,
		})
	}

	return domain.NewDispatchPolicy().RankCandidates(candidatesSlice), nil
}

// ReserveDriver reserves a driver for a delivery using distributed locking and conditional state transition.
func (s *DispatchService) ReserveDriver(ctx context.Context, driverID, deliveryID string) (bool, error) {
	// Acquire distributed lock for the driver
	acquired, err := s.lockManager.Acquire(ctx, "driver:"+driverID, 30*time.Second)
	if err != nil {
		log.Printf("dispatch service: failed to acquire lock for driver %s: %v", driverID, err)
		return false, err
	}
	if !acquired {
		log.Printf("dispatch service: lock already held for driver %s", driverID)
		return false, nil
	}
	defer s.lockManager.Release(ctx, "driver:"+driverID, "temp-token")

	// Check driver state from MongoDB
	driver, err := s.driverRepo.FindByID(ctx, driverID)
	if err != nil {
		log.Printf("dispatch service: failed to find driver %s: %v", driverID, err)
		return false, err
	}
	if driver == nil {
		log.Printf("dispatch service: driver %s not found", driverID)
		return false, domain.ErrDriverNotFound
	}

	// Check driver is available
	if driver.Status != domain.DriverStatusAvailable {
		log.Printf("dispatch service: driver %s is not available, status: %s", driverID, driver.Status)
		return false, domain.ErrDriverNotAvailableForAssignment
	}

	// Conditionally update driver state from AVAILABLE to BUSY
	err = s.driverRepo.Save(ctx, driver)
	if err != nil {
		log.Printf("dispatch service: failed to update driver %s state: %v", driverID, err)
		return false, err
	}

	// Create assignment record as OFFERED
	assignment := domain.Assignment{
		ID:         deliveryID + "-" + driverID,
		DriverID:   driverID,
		DeliveryID: deliveryID,
		Status:     domain.AssignmentStatusOffered,
		AttemptNumber: 1,
		OfferedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(20 * time.Second), // default 20s timeout
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = s.assignmentRepo.Save(ctx, assignment.ID, assignment.DriverID, assignment.DeliveryID, string(assignment.Status))
	if err != nil {
		log.Printf("dispatch service: failed to save assignment %s: %v", assignment.ID, err)
		// Reset driver state on failure
		driver.Status = domain.DriverStatusAvailable
		s.driverRepo.Save(ctx, driver)
		return false, err
	}

	// Publish assignment offered event
	err = s.eventPublisher.PublishAssignmentOffered(ctx, assignment.ID, assignment.DeliveryID, assignment.DriverID)
	if err != nil {
		log.Printf("dispatch service: failed to publish assignment offered event: %v", err)
		// Don't fail the whole operation - event publishing is best-effort
	}

	log.Printf("dispatch service: driver %s reserved for delivery %s", driverID, deliveryID)
	return true, nil
}

// ReleaseDriver releases a driver from their current assignment.
func (s *DispatchService) ReleaseDriver(ctx context.Context, driverID, deliveryID string) error {
	// Find active assignment for driver
	assignmentID, found := s.assignmentRepo.FindActiveByDriver(ctx, driverID)
	if !found {
		log.Printf("dispatch service: no active assignment found for driver %s", driverID)
		return domain.ErrAssignmentNotFound
	}

	// Update assignment status to RELEASED/COMPLETED
	err := s.assignmentRepo.UpdateStatus(ctx, assignmentID, string(domain.AssignmentStatusCompleted))
	if err != nil {
		log.Printf("dispatch service: failed to update assignment %s status: %v", assignmentID, err)
		return err
	}

	// Update driver state back to AVAILABLE
	driver, err := s.driverRepo.FindByID(ctx, driverID)
	if err != nil {
		log.Printf("dispatch service: failed to find driver %s: %v", driverID, err)
		return err
	}
	if driver != nil {
		driver.Status = domain.DriverStatusAvailable
		driver.UpdatedAt = time.Now()
		err = s.driverRepo.Save(ctx, driver)
		if err != nil {
			log.Printf("dispatch service: failed to restore driver %s state: %v", driverID, err)
			return err
		}
	}

	// Publish driver available event
	err = s.eventPublisher.PublishDriverAvailable(ctx, driverID)
	if err != nil {
		log.Printf("dispatch service: failed to publish driver available event: %v", err)
	}

	log.Printf("dispatch service: driver %s released from delivery %s", driverID, deliveryID)
	return nil
}

// AcceptAssignment accepts a driver's assignment offer.
func (s *DispatchService) AcceptAssignment(ctx context.Context, assignmentID, driverID string) error {
	// Validate assignment state is OFFERED
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
		return domain.ErrAssignmentInvalidState
	}

	// Transition assignment from OFFERED to ACCEPTED
	now := time.Now()
	assignment.Status = domain.AssignmentStatusAccepted
	assignment.AcceptedAt = &now
	assignment.UpdatedAt = time.Now()

	err = s.assignmentRepo.Save(ctx, assignment.ID, assignment.DriverID, assignment.DeliveryID, string(assignment.Status))
	if err != nil {
		log.Printf("dispatch service: failed to save assignment %s: %v", assignmentID, err)
		return err
	}

	// Update driver state from BUSY to... keep BUSY until delivery completes, or transition
	// For now, driver remains BUSY as they've accepted the assignment

	// Publish assignment accepted event
	err = s.eventPublisher.PublishAssignmentAccepted(ctx, assignment.ID, assignment.DeliveryID, assignment.DriverID)
	if err != nil {
		log.Printf("dispatch service: failed to publish assignment accepted event: %v", err)
	}

	log.Printf("dispatch service: assignment %s accepted by driver %s", assignmentID, driverID)
	return nil
}

// RejectAssignment rejects a driver's assignment offer.
func (s *DispatchService) RejectAssignment(ctx context.Context, assignmentID, driverID, reason string) error {
	// Find assignment
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
		return domain.ErrAssignmentInvalidState
	}

	// Transition assignment from OFFERED to REJECTED
	now := time.Now()
	assignment.Status = domain.AssignmentStatusRejected
	assignment.RejectedAt = &now
	assignment.UpdatedAt = time.Now()

	err = s.assignmentRepo.Save(ctx, assignment.ID, assignment.DriverID, assignment.DeliveryID, string(assignment.Status))
	if err != nil {
		log.Printf("dispatch service: failed to save assignment %s: %v", assignmentID, err)
		return err
	}

	// Release driver back to AVAILABLE
	driver, err := s.driverRepo.FindByID(ctx, driverID)
	if err != nil {
		log.Printf("dispatch service: failed to find driver %s: %v", driverID, err)
		return err
	}
	if driver != nil {
		driver.Status = domain.DriverStatusAvailable
		driver.UpdatedAt = time.Now()
		err = s.driverRepo.Save(ctx, driver)
		if err != nil {
			log.Printf("dispatch service: failed to restore driver %s state: %v", driverID, err)
			return err
		}
	}

	// Record dispatch attempt
	dispatchP := domain.NewDispatchPolicy()
	dispatchP.RecordAttempt(domain.DispatchAttempt{
		AttemptNumber: 1,
		DriverID:      driverID,
		Result:        domain.DispatchResultRejected,
		Reason:        reason,
		CreatedAt:     time.Now(),
	})

	// Publish assignment rejected event
	err = s.eventPublisher.PublishAssignmentRejected(ctx, assignment.ID, assignment.DeliveryID, driverID, reason)
	if err != nil {
		log.Printf("dispatch service: failed to publish assignment rejected event: %v", err)
	}

	log.Printf("dispatch service: assignment %s rejected by driver %s", assignmentID, driverID)
	return nil
}

// ReleaseDriverByID releases a driver assignment by ID.
func (s *DispatchService) ReleaseDriverByID(ctx context.Context, assignmentID string) error {
	// Find assignment
	assignment, err := s.assignmentRepo.FindByID(ctx, assignmentID)
	if err != nil {
		return err
	}
	if assignment == nil {
		return domain.ErrAssignmentNotFound
	}

	// Transition assignment based on current state
	switch assignment.Status {
	case domain.AssignmentStatusActive:
		assignment.Status = domain.AssignmentStatusCompleted
	case domain.AssignmentStatusOffered:
		assignment.Status = domain.AssignmentStatusExpired // or released
	default:
		// Already in terminal state, no-op
	}

	assignment.UpdatedAt = time.Now()
	err = s.assignmentRepo.Save(ctx, assignment.ID, assignment.DriverID, assignment.DeliveryID, string(assignment.Status))
	if err != nil {
		log.Printf("dispatch service: failed to save assignment %s: %v", assignmentID, err)
		return err
	}

	// Release driver back to AVAILABLE if was offered
	if assignment.Status == domain.AssignmentStatusExpired || assignment.Status == domain.AssignmentStatusCompleted {
		driver, err := s.driverRepo.FindByID(ctx, assignment.DriverID)
		if err != nil {
			log.Printf("dispatch service: failed to find driver %s: %v", assignment.DriverID, err)
			return err
		}
		if driver != nil {
			driver.Status = domain.DriverStatusAvailable
			driver.UpdatedAt = time.Now()
			err = s.driverRepo.Save(ctx, driver)
			if err != nil {
				log.Printf("dispatch service: failed to restore driver %s state: %v", assignment.DriverID, err)
				return err
			}
			// Publish driver available event
			err = s.eventPublisher.PublishDriverAvailable(ctx, assignment.DriverID)
			if err != nil {
				log.Printf("dispatch service: failed to publish driver available event: %v", err)
			}
		}
	}

	log.Printf("dispatch service: assignment %s released", assignmentID)
	return nil
}

// ValidateDriverID validates a driver ID using the driver repository.
func (s *DispatchService) ValidateDriverID(ctx context.Context, driverID string) (bool, string) {
	driver, err := s.driverRepo.FindByID(ctx, driverID)
	if err != nil {
		log.Printf("dispatch service: failed to validate driver %s: %v", driverID, err)
		return false, ""
	}
	if driver == nil {
		return false, ""
	}
	return true, string(driver.Status)
}