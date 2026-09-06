package workers

import (
	"context"
	"log"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// AssignmentExpiryWorker periodically checks for expired assignment offers and releases them.
type AssignmentExpiryWorker struct {
	assignmentRepo ports.AssignmentRepository
	driverRepo     ports.DriverRepository
	eventPublisher ports.EventPublisher
	stopChan       chan struct{}
}

// NewAssignmentExpiryWorker creates a new assignment expiry worker.
func NewAssignmentExpiryWorker(assignmentRepo ports.AssignmentRepository,
	driverRepo ports.DriverRepository, eventPublisher ports.EventPublisher) *AssignmentExpiryWorker {

	return &AssignmentExpiryWorker{
		assignmentRepo:  assignmentRepo,
		driverRepo:     driverRepo,
		eventPublisher: eventPublisher,
		stopChan:       make(chan struct{}),
	}
}

// Start begins the expiry checking loop.
func (w *AssignmentExpiryWorker) Start(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-w.stopChan:
				return
			case <-ticker.C:
				w.checkExpiredAssignments(ctx)
			}
		}
	}()
}

// Stop stops the worker.
func (w *AssignmentExpiryWorker) Stop() {
	close(w.stopChan)
}

func (w *AssignmentExpiryWorker) checkExpiredAssignments(ctx context.Context) {
	// Find assignments OFFERED past their expiresAt timeout
	expiredAssignments, err := w.assignmentRepo.ExpireOffers(ctx, 5*time.Second)
	if err != nil {
		log.Printf("assignment_expiry: failed to expire offers: %v", err)
		return
	}

	for _, assignmentID := range expiredAssignments {
		// Release the driver back to AVAILABLE
		// Find assignment details
		assignment, err := w.assignmentRepo.FindByID(ctx, assignmentID)
		if err != nil || assignment == nil {
			log.Printf("assignment_expiry: failed to find expired assignment %s: %v", assignmentID, err)
			continue
		}

		// Update assignment status to EXPIRED
		err = w.assignmentRepo.UpdateStatus(ctx, assignmentID, string(domain.AssignmentStatusExpired))
		if err != nil {
			log.Printf("assignment_expiry: failed to update assignment %s to EXPIRED: %v", assignmentID, err)
			continue
		}

		// Release driver back to AVAILABLE
		driver, err := w.driverRepo.FindByID(ctx, assignment.DriverID)
		if err != nil || driver == nil {
			log.Printf("assignment_expiry: failed to find driver %s: %v", assignment.DriverID, err)
			continue
		}
		driver.Status = domain.DriverStatusAvailable
		driver.UpdatedAt = time.Now()
		w.driverRepo.Save(ctx, driver)

		// Publish driver available event
		w.eventPublisher.PublishDriverAvailable(ctx, assignment.DriverID)

		log.Printf("assignment_expiry: assignment %s expired, driver %s released", assignmentID, assignment.DriverID)
	}
}