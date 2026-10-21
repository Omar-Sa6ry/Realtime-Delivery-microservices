package workers

import (
	"context"
	"log"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

type DispatchServiceForWorkers interface {
	ReleaseDriver(ctx context.Context, driverID, deliveryID string) error
	ReleaseDriverByAssignment(ctx context.Context, assignmentID string) error
}

type AssignmentExpiryWorker struct {
	assignmentRepo ports.AssignmentRepository
	dispatch       DispatchServiceForWorkers
}

func NewAssignmentExpiryWorker(assignmentRepo ports.AssignmentRepository, dispatch DispatchServiceForWorkers) *AssignmentExpiryWorker {
	return &AssignmentExpiryWorker{
		assignmentRepo: assignmentRepo,
		dispatch:       dispatch,
	}
}

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
		if w.dispatch != nil {
			if err := w.dispatch.ReleaseDriverByAssignment(ctx, assignmentID); err != nil {
				log.Printf("assignment_expiry: failed to release driver by assignment %s: %v", assignmentID, err)
			} else {
				log.Printf("assignment_expiry: successfully released driver and published expired event for assignment %s", assignmentID)
			}
		} else {
			if err = w.assignmentRepo.UpdateStatus(ctx, assignmentID, string(domain.AssignmentStatusExpired)); err != nil {
				log.Printf("assignment_expiry: failed to update assignment %s to EXPIRED: %v", assignmentID, err)
			}
		}
	}
}
