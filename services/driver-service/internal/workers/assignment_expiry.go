package workers

import (
	"context"
	"log"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// DispatchServiceForWorkers is a minimal interface that workers need from DispatchService.
type DispatchServiceForWorkers interface {
	ReleaseDriver(ctx context.Context, driverID, deliveryID string) error
}

// AssignmentExpiryWorker periodically checks for expired assignment offers and releases them.
type AssignmentExpiryWorker struct {
	assignmentRepo ports.AssignmentRepository
	driverRepo     ports.DriverRepository
	eventPublisher ports.EventPublisher
}

func NewAssignmentExpiryWorker(assignmentRepo ports.AssignmentRepository, dispatch DispatchServiceForWorkers) *AssignmentExpiryWorker {
	return &AssignmentExpiryWorker{
		assignmentRepo: assignmentRepo,
	}
}

// Run starts the expiry checking loop and blocks until ctx is cancelled.
func (w *AssignmentExpiryWorker) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Printf("assignment_expiry: worker started (interval: %s)", interval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("assignment_expiry: worker stopped")
			return
		case <-ticker.C:
			w.checkExpiredAssignments(ctx)
		}
	}
}

func (w *AssignmentExpiryWorker) checkExpiredAssignments(ctx context.Context) {
	expiredIDs, err := w.assignmentRepo.ExpireOffers(ctx, 0) // ExpireOffers checks expiresAt internally
	if err != nil {
		log.Printf("assignment_expiry: failed to expire offers: %v", err)
		return
	}

	for _, assignmentID := range expiredIDs {
		assignment, err := w.assignmentRepo.FindByID(ctx, assignmentID)
		if err != nil || assignment == nil {
			log.Printf("assignment_expiry: failed to find expired assignment %s: %v", assignmentID, err)
			continue
		}

		if err = w.assignmentRepo.UpdateStatus(ctx, assignmentID, string(domain.AssignmentStatusExpired)); err != nil {
			log.Printf("assignment_expiry: failed to update assignment %s to EXPIRED: %v", assignmentID, err)
		}

		log.Printf("assignment_expiry: assignment %s expired (driver %s)", assignmentID, assignment.DriverID)
	}
}