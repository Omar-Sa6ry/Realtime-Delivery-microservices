package workers

import (
	"context"
	"log"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/mongodb"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// ReconciliationWorker periodically inspects and repairs inconsistent state in the system.
type ReconciliationWorker struct {
	driverRepo      ports.DriverRepository
	assignmentRepo  ports.AssignmentRepository
	locationStore   ports.LocationStore
	eventPublisher  ports.EventPublisher
	mongodbRepo     *mongodb.DriverRepository
	stopChan        chan struct{}
}

// NewReconciliationWorker creates a new reconciliation worker.
func NewReconciliationWorker(driverRepo ports.DriverRepository, assignmentRepo ports.AssignmentRepository,
	locationStore ports.LocationStore, eventPublisher ports.EventPublisher, mongodbRepo *mongodb.DriverRepository) *ReconciliationWorker {

	return &ReconciliationWorker{
		driverRepo:      driverRepo,
		assignmentRepo:  assignmentRepo,
		locationStore:   locationStore,
		eventPublisher:  eventPublisher,
		mongodbRepo:     mongodbRepo,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the reconciliation loop.
func (w *ReconciliationWorker) Start(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-w.stopChan:
				return
			case <-ticker.C:
				w.reconcile(ctx)
			}
		}
	}()
}

// Stop stops the worker.
func (w *ReconciliationWorker) Stop() {
	close(w.stopChan)
}

func (w *ReconciliationWorker) reconcile(ctx context.Context) {
	// 1. Check for drivers marked BUSY without active assignment
	busyDrivers, err := w.driverRepo.FindByStatus(ctx, string(domain.DriverStatusBusy))
	if err != nil {
		log.Printf("reconciliation: failed to find BUSY drivers: %v", err)
		return
	}

	for _, driver := range busyDrivers {
		// Check if driver has an active assignment
		activeAssignmentID, found := w.assignmentRepo.FindActiveByDriver(ctx, driver.ID)
		if !found || activeAssignmentID == "" {
			// Driver is BUSY but has no active assignment - repair by making AVAILABLE
			log.Printf("reconciliation: driver %s is BUSY with no active assignment, repairing", driver.ID)
			driver.Status = domain.DriverStatusAvailable
			driver.UpdatedAt = time.Now()
			w.driverRepo.Save(ctx, driver)

			// Publish driver available event
			w.eventPublisher.PublishDriverAvailable(ctx, driver.ID)
		} else {
			// Assignment exists, check if it's still valid
			assignment, err := w.assignmentRepo.FindByID(ctx, activeAssignmentID)
			if err != nil || assignment == nil {
				log.Printf("reconciliation: assignment %s not found for driver %s, repairing", activeAssignmentID, driver.ID)
				driver.Status = domain.DriverStatusAvailable
				driver.UpdatedAt = time.Now()
				w.driverRepo.Save(ctx, driver)
				w.eventPublisher.PublishDriverAvailable(ctx, driver.ID)
			}
		}
	}

	// 2. Check for assignments OFFERED past expiresAt (using expired path)
	// This is handled by the assignment expiry worker, but we can also check here

	// 3. Check for Redis GEO entries for OFFLINE drivers
	// (would need to query Redis - skipped for now as it requires direct Redis access)

	// 4. Check for stale locks
	// (would need to check Redis lock TTLs - skipped for now)

	log.Printf("reconciliation: completed state repair checks")
}