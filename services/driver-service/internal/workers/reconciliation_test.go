package workers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

// assignmentRepoWithFindError injects a failure into FindActiveByDriver,
// which the shared mock cannot produce.
type assignmentRepoWithFindError struct {
	*mocks.MockAssignmentRepository
	findErr error
}

func (r *assignmentRepoWithFindError) FindActiveByDriver(ctx context.Context, driverID string) (*domain.Assignment, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.MockAssignmentRepository.FindActiveByDriver(ctx, driverID)
}

func busyDriver(id string) *domain.Driver {
	return &domain.Driver{
		ID:        id,
		UserID:    "user-" + id,
		Status:    domain.DriverStatusBusy,
		IsActive:  true,
		IsBlocked: false,
		Vehicle:   domain.VehicleInfo{Type: domain.VehicleTypeCar},
	}
}

func TestReconciliationWorker_reconcile(t *testing.T) {
	t.Run("no busy drivers - nothing to do", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetDriver(&domain.Driver{ID: "d1", Status: domain.DriverStatusAvailable, IsActive: true})
		assignmentRepo := mocks.NewMockAssignmentRepository()

		w := NewReconciliationWorker(driverRepo, assignmentRepo)
		w.reconcile(context.Background())

		driver, _ := driverRepo.FindByID(context.Background(), "d1")
		if driver == nil || driver.Status != domain.DriverStatusAvailable {
			t.Errorf("driver = %+v, want AVAILABLE", driver)
		}
	})

	t.Run("busy driver without active assignment is repaired", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetDriver(busyDriver("d1"))
		assignmentRepo := mocks.NewMockAssignmentRepository()

		w := NewReconciliationWorker(driverRepo, assignmentRepo)
		w.reconcile(context.Background())

		driver, _ := driverRepo.FindByID(context.Background(), "d1")
		if driver == nil || driver.Status != domain.DriverStatusAvailable {
			t.Errorf("driver = %+v, want AVAILABLE after repair", driver)
		}
	})

	t.Run("busy driver with active assignment is left alone", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetDriver(busyDriver("d1"))
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(&domain.Assignment{
			ID: "a1", DriverID: "d1", DeliveryID: "delivery-1", Status: domain.AssignmentStatusActive,
		})

		w := NewReconciliationWorker(driverRepo, assignmentRepo)
		w.reconcile(context.Background())

		driver, _ := driverRepo.FindByID(context.Background(), "d1")
		if driver == nil || driver.Status != domain.DriverStatusBusy {
			t.Errorf("driver = %+v, want BUSY (untouched)", driver)
		}
	})

	t.Run("active assignment lookup error triggers repair", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetDriver(busyDriver("d1"))
		assignmentRepo := &assignmentRepoWithFindError{
			MockAssignmentRepository: mocks.NewMockAssignmentRepository(),
			findErr:                  errors.New("mongo down"),
		}

		w := NewReconciliationWorker(driverRepo, assignmentRepo)
		w.reconcile(context.Background())

		driver, _ := driverRepo.FindByID(context.Background(), "d1")
		if driver == nil || driver.Status != domain.DriverStatusAvailable {
			t.Errorf("driver = %+v, want AVAILABLE after repair", driver)
		}
	})

	t.Run("driver lookup error is tolerated", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetError(errors.New("mongo down"))
		assignmentRepo := mocks.NewMockAssignmentRepository()

		w := NewReconciliationWorker(driverRepo, assignmentRepo)
		w.reconcile(context.Background()) // must not panic
	})
}

func TestReconciliationWorker_Run(t *testing.T) {
	driverRepo := mocks.NewMockDriverRepository()
	driverRepo.SetDriver(busyDriver("d1"))
	assignmentRepo := mocks.NewMockAssignmentRepository()

	w := NewReconciliationWorker(driverRepo, assignmentRepo)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx, 2*time.Millisecond)
		close(done)
	}()

	waitUntil(t, func() bool {
		driver, err := driverRepo.FindByID(context.Background(), "d1")
		return err == nil && driver != nil && driver.Status == domain.DriverStatusAvailable
	}, 3*time.Second, "reconciliation worker never repaired the busy driver")

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}
