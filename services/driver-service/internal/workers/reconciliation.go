package workers

import (
	"context"
	"log"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// ReconciliationWorker periodically inspects and repairs inconsistent state in the system.
type ReconciliationWorker struct {
	driverRepo     ports.DriverRepository
	assignmentRepo ports.AssignmentRepository
}

// NewReconciliationWorker creates a new reconciliation worker.
func NewReconciliationWorker(driverRepo ports.DriverRepository, assignmentRepo ports.AssignmentRepository) *ReconciliationWorker {
	return &ReconciliationWorker{
		driverRepo:     driverRepo,
		assignmentRepo: assignmentRepo,
	}
}

// Run starts the reconciliation loop and blocks until ctx is cancelled.
func (w *ReconciliationWorker) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Printf("reconciliation: worker started (interval: %s)", interval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("reconciliation: worker stopped")
			return
		case <-ticker.C:
			w.reconcile(ctx)
		}
	}
}

func (w *ReconciliationWorker) reconcile(ctx context.Context) {
	// Check for drivers marked BUSY without active assignment
	busyDrivers, err := w.driverRepo.FindByStatus(ctx, string(domain.DriverStatusBusy))
	if err != nil {
		log.Printf("reconciliation: failed to find BUSY drivers: %v", err)
		return
	}

	for _, driver := range busyDrivers {
		assignment, err := w.assignmentRepo.FindActiveByDriver(ctx, driver.ID)
		if err != nil || assignment == nil {
			// Driver is BUSY but has no active assignment — repair
			log.Printf("reconciliation: driver %s is BUSY with no active assignment, repairing", driver.ID)
			driver.Status = domain.DriverStatusAvailable
			driver.UpdatedAt = time.Now()
			_ = w.driverRepo.Save(ctx, driver)
		}
	}

	log.Printf("reconciliation: completed state repair checks")
}