package commands_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/commands"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// ---------------------------------------------------------------------------
// Local test doubles: inject failures that the shared mocks cannot produce
// (e.g. Save failing while FindByID still succeeds).
// ---------------------------------------------------------------------------

type driverRepoWithSaveError struct {
	*mocks.MockDriverRepository
	saveErr error
}

func (r *driverRepoWithSaveError) Save(ctx context.Context, driver *domain.Driver) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	return r.MockDriverRepository.Save(ctx, driver)
}

type assignmentRepoWithSaveError struct {
	*mocks.MockAssignmentRepository
	saveErr error
}

func (r *assignmentRepoWithSaveError) Save(ctx context.Context, assignment *domain.Assignment) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	return r.MockAssignmentRepository.Save(ctx, assignment)
}

type lockManagerWithAcquireError struct {
	*mocks.MockLockManager
	acquireErr error
}

func (r *lockManagerWithAcquireError) Acquire(ctx context.Context, resource string, ttl time.Duration) (bool, error) {
	if r.acquireErr != nil {
		return false, r.acquireErr
	}
	return r.MockLockManager.Acquire(ctx, resource, ttl)
}

type reviewRepoWithErrors struct {
	*mocks.MockReviewRepository
	findDeliveryErr error
	avgErr          error
}

func (r *reviewRepoWithErrors) FindByDeliveryID(ctx context.Context, deliveryID string) (*domain.Review, error) {
	if r.findDeliveryErr != nil {
		return nil, r.findDeliveryErr
	}
	return r.MockReviewRepository.FindByDeliveryID(ctx, deliveryID)
}

func (r *reviewRepoWithErrors) CalculateAverageRating(ctx context.Context, driverID string) (float64, int64, error) {
	if r.avgErr != nil {
		return 0, 0, r.avgErr
	}
	return r.MockReviewRepository.CalculateAverageRating(ctx, driverID)
}

func availableDriver(id string) *domain.Driver {
	return &domain.Driver{
		ID:        id,
		UserID:    "user-" + id,
		Status:    domain.DriverStatusAvailable,
		IsActive:  true,
		IsBlocked: false,
		Vehicle:   domain.VehicleInfo{Type: domain.VehicleTypeCar, PlateNumber: "ABC123", CapacityKg: 100},
	}
}

func offeredAssignment(id, driverID, deliveryID string) *domain.Assignment {
	return &domain.Assignment{
		ID:         id,
		DriverID:   driverID,
		DeliveryID: deliveryID,
		Status:     domain.AssignmentStatusOffered,
	}
}

// ---------------------------------------------------------------------------
// Accept / Reject / Release / GoOnline / GoOffline / Block / Unblock
// ---------------------------------------------------------------------------

func TestAcceptAssignmentCommand_SaveAndPublishFailures(t *testing.T) {
	t.Run("assignment save error", func(t *testing.T) {
		base := mocks.NewMockAssignmentRepository()
		base.SetAssignment(offeredAssignment("a1", "driver-1", "delivery-1"))
		repo := &assignmentRepoWithSaveError{MockAssignmentRepository: base, saveErr: errors.New("save failed")}
		pub := mocks.NewMockEventPublisher()

		cmd := commands.NewAcceptAssignmentCommand("a1", "driver-1", repo, pub)
		if err := cmd.Execute(context.Background()); err == nil || err.Error() != "save failed" {
			t.Errorf("Execute() error = %v, want save failed", err)
		}
	})

	t.Run("publish failure is non-fatal", func(t *testing.T) {
		base := mocks.NewMockAssignmentRepository()
		base.SetAssignment(offeredAssignment("a1", "driver-1", "delivery-1"))
		pub := mocks.NewMockEventPublisher()
		pub.SetError("AssignmentAccepted", errors.New("kafka down"))

		cmd := commands.NewAcceptAssignmentCommand("a1", "driver-1", base, pub)
		if err := cmd.Execute(context.Background()); err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}
		assignment, _ := base.FindByID(context.Background(), "a1")
		if assignment == nil || assignment.Status != domain.AssignmentStatusAccepted {
			t.Errorf("assignment = %+v, want ACCEPTED", assignment)
		}
	})
}

func TestRejectAssignmentCommand_SaveAndPublishFailures(t *testing.T) {
	t.Run("assignment save error", func(t *testing.T) {
		base := mocks.NewMockAssignmentRepository()
		base.SetAssignment(offeredAssignment("a1", "driver-1", "delivery-1"))
		repo := &assignmentRepoWithSaveError{MockAssignmentRepository: base, saveErr: errors.New("save failed")}
		pub := mocks.NewMockEventPublisher()

		cmd := commands.NewRejectAssignmentCommand("a1", "driver-1", "too far", repo, pub)
		if err := cmd.Execute(context.Background()); err == nil || err.Error() != "save failed" {
			t.Errorf("Execute() error = %v, want save failed", err)
		}
	})

	t.Run("publish failure is non-fatal", func(t *testing.T) {
		base := mocks.NewMockAssignmentRepository()
		base.SetAssignment(offeredAssignment("a1", "driver-1", "delivery-1"))
		pub := mocks.NewMockEventPublisher()
		pub.SetError("AssignmentRejected", errors.New("kafka down"))

		cmd := commands.NewRejectAssignmentCommand("a1", "driver-1", "too far", base, pub)
		if err := cmd.Execute(context.Background()); err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}
		assignment, _ := base.FindByID(context.Background(), "a1")
		if assignment == nil || assignment.Status != domain.AssignmentStatusRejected {
			t.Errorf("assignment = %+v, want REJECTED", assignment)
		}
	})
}

func TestReleaseDriverCommand_SaveAndPublishFailures(t *testing.T) {
	t.Run("driver save error", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(&domain.Assignment{
			ID: "a1", DriverID: "driver-1", DeliveryID: "delivery-1", Status: domain.AssignmentStatusActive,
		})
		baseDriverRepo := mocks.NewMockDriverRepository()
		baseDriverRepo.SetDriver(availableDriver("driver-1"))
		driverRepo := &driverRepoWithSaveError{MockDriverRepository: baseDriverRepo, saveErr: errors.New("save failed")}
		pub := mocks.NewMockEventPublisher()

		cmd := commands.NewReleaseDriverCommand("driver-1", "delivery-1", driverRepo, assignmentRepo, pub)
		if err := cmd.Execute(context.Background()); err == nil || err.Error() != "save failed" {
			t.Errorf("Execute() error = %v, want save failed", err)
		}
	})

	t.Run("publish failure is non-fatal", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(&domain.Assignment{
			ID: "a1", DriverID: "driver-1", DeliveryID: "delivery-1", Status: domain.AssignmentStatusActive,
		})
		baseDriverRepo := mocks.NewMockDriverRepository()
		baseDriverRepo.SetDriver(availableDriver("driver-1"))
		pub := mocks.NewMockEventPublisher()
		pub.SetError("DriverAvailable", errors.New("kafka down"))

		cmd := commands.NewReleaseDriverCommand("driver-1", "delivery-1", baseDriverRepo, assignmentRepo, pub)
		if err := cmd.Execute(context.Background()); err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}
	})
}

func TestReserveDriverCommand_ExtraFailures(t *testing.T) {
	t.Run("lock acquire error", func(t *testing.T) {
		lock := &lockManagerWithAcquireError{MockLockManager: mocks.NewMockLockManager(), acquireErr: errors.New("redis down")}
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetDriver(availableDriver("driver-1"))
		assignmentRepo := mocks.NewMockAssignmentRepository()
		pub := mocks.NewMockEventPublisher()

		cmd := commands.NewReserveDriverCommand("driver-1", "delivery-1", driverRepo, assignmentRepo, lock, pub, domain.NewDispatchPolicy())
		reserved, err := cmd.Execute(context.Background())
		if err == nil || err.Error() != "redis down" {
			t.Errorf("Execute() error = %v, want redis down", err)
		}
		if reserved {
			t.Error("Execute() reserved = true, want false")
		}
	})

	t.Run("driver save error", func(t *testing.T) {
		baseDriverRepo := mocks.NewMockDriverRepository()
		baseDriverRepo.SetDriver(availableDriver("driver-1"))
		driverRepo := &driverRepoWithSaveError{MockDriverRepository: baseDriverRepo, saveErr: errors.New("save failed")}
		assignmentRepo := mocks.NewMockAssignmentRepository()
		lock := mocks.NewMockLockManager()
		pub := mocks.NewMockEventPublisher()

		cmd := commands.NewReserveDriverCommand("driver-1", "delivery-1", driverRepo, assignmentRepo, lock, pub, domain.NewDispatchPolicy())
		reserved, err := cmd.Execute(context.Background())
		if err == nil || err.Error() != "save failed" {
			t.Errorf("Execute() error = %v, want save failed", err)
		}
		if reserved {
			t.Error("Execute() reserved = true, want false")
		}
	})

	t.Run("publish failure is non-fatal", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetDriver(availableDriver("driver-1"))
		assignmentRepo := mocks.NewMockAssignmentRepository()
		lock := mocks.NewMockLockManager()
		pub := mocks.NewMockEventPublisher()
		pub.SetError("AssignmentOffered", errors.New("kafka down"))

		cmd := commands.NewReserveDriverCommand("driver-1", "delivery-1", driverRepo, assignmentRepo, lock, pub, domain.NewDispatchPolicy())
		reserved, err := cmd.Execute(context.Background())
		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}
		if !reserved {
			t.Error("Execute() reserved = false, want true")
		}
	})
}

func TestGoOfflineCommand_SaveError(t *testing.T) {
	base := mocks.NewMockDriverRepository()
	base.SetDriver(availableDriver("driver-1"))
	repo := &driverRepoWithSaveError{MockDriverRepository: base, saveErr: errors.New("save failed")}
	pub := mocks.NewMockEventPublisher()

	cmd := commands.NewGoOfflineCommand("driver-1", repo, pub)
	if err := cmd.Execute(context.Background()); err == nil || err.Error() != "save failed" {
		t.Errorf("Execute() error = %v, want save failed", err)
	}
}

func TestGoOnlineCommand_SaveAndPublishFailures(t *testing.T) {
	t.Run("driver save error", func(t *testing.T) {
		base := mocks.NewMockDriverRepository()
		driver := availableDriver("driver-1")
		driver.Status = domain.DriverStatusOffline
		base.SetDriver(driver)
		repo := &driverRepoWithSaveError{MockDriverRepository: base, saveErr: errors.New("save failed")}
		pub := mocks.NewMockEventPublisher()

		cmd := commands.NewGoOnlineCommand("driver-1", repo, pub)
		if err := cmd.Execute(context.Background()); err == nil || err.Error() != "save failed" {
			t.Errorf("Execute() error = %v, want save failed", err)
		}
	})

	t.Run("publish failure is non-fatal", func(t *testing.T) {
		base := mocks.NewMockDriverRepository()
		driver := availableDriver("driver-1")
		driver.Status = domain.DriverStatusOffline
		base.SetDriver(driver)
		pub := mocks.NewMockEventPublisher()
		pub.SetError("DriverAvailable", errors.New("kafka down"))

		cmd := commands.NewGoOnlineCommand("driver-1", base, pub)
		if err := cmd.Execute(context.Background()); err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}
		saved, _ := base.FindByID(context.Background(), "driver-1")
		if saved == nil || saved.Status != domain.DriverStatusAvailable {
			t.Errorf("driver = %+v, want AVAILABLE", saved)
		}
	})
}

func TestBlockDriverHandler_SaveError(t *testing.T) {
	base := mocks.NewMockDriverRepository()
	base.SetDriver(availableDriver("driver-1"))
	repo := &driverRepoWithSaveError{MockDriverRepository: base, saveErr: errors.New("save failed")}

	handler := commands.NewBlockDriverHandler(repo, mocks.NewMockEventPublisher())
	if _, err := handler.Execute(context.Background(), commands.BlockDriverCommand{DriverID: "driver-1", Reason: "spam"}); err == nil || err.Error() != "save failed" {
		t.Errorf("Execute() error = %v, want save failed", err)
	}
}

func TestUnblockDriverHandler_SaveError(t *testing.T) {
	base := mocks.NewMockDriverRepository()
	driver := availableDriver("driver-1")
	driver.IsBlocked = true
	base.SetDriver(driver)
	repo := &driverRepoWithSaveError{MockDriverRepository: base, saveErr: errors.New("save failed")}

	handler := commands.NewUnblockDriverHandler(repo, mocks.NewMockEventPublisher())
	if _, err := handler.Execute(context.Background(), commands.UnblockDriverCommand{DriverID: "driver-1"}); err == nil || err.Error() != "save failed" {
		t.Errorf("Execute() error = %v, want save failed", err)
	}
}

// ---------------------------------------------------------------------------
// RateDriver error branches
// ---------------------------------------------------------------------------

func TestRateDriverHandler_ErrorBranches(t *testing.T) {
	baseDriver := availableDriver("driver-1")
	delivery := &ports.DeliveryInfo{
		DeliveryID: "delivery-1",
		CustomerID: "user-1",
		DriverID:   "driver-1",
	}

	t.Run("driver lookup error", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetError(errors.New("mongo down"))
		handler := commands.NewRateDriverHandler(driverRepo, mocks.NewMockReviewRepository(), mocks.NewMockDeliveryServiceClient(), mocks.NewMockAssignmentRepository())

		_, err := handler.Execute(context.Background(), commands.RateDriverCommand{
			DriverID: "driver-1", UserID: "user-1", DeliveryID: "delivery-1", Rating: 5,
		})
		if err == nil || err.Error() != "mongo down" {
			t.Errorf("Execute() error = %v, want mongo down", err)
		}
	})

	t.Run("delivery client error", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetDriver(baseDriver)
		deliveryClient := mocks.NewMockDeliveryServiceClient()
		deliveryClient.SetError("delivery-1", errors.New("grpc down"))
		handler := commands.NewRateDriverHandler(driverRepo, mocks.NewMockReviewRepository(), deliveryClient, mocks.NewMockAssignmentRepository())

		_, err := handler.Execute(context.Background(), commands.RateDriverCommand{
			DriverID: "driver-1", UserID: "user-1", DeliveryID: "delivery-1", Rating: 5,
		})
		if err == nil || err.Error() != "grpc down" {
			t.Errorf("Execute() error = %v, want grpc down", err)
		}
	})

	t.Run("assignment repo fallback matches driver", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetDriver(baseDriver)
		deliveryClient := mocks.NewMockDeliveryServiceClient()
		// delivery record points at a different driver → forces the assignment lookup
		deliveryClient.SetDelivery(&ports.DeliveryInfo{
			DeliveryID: "delivery-1",
			CustomerID: "user-1",
			DriverID:   "driver-9",
		})
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(&domain.Assignment{
			ID: "a1", DriverID: "driver-1", DeliveryID: "delivery-1", Status: domain.AssignmentStatusCompleted,
		})
		handler := commands.NewRateDriverHandler(driverRepo, mocks.NewMockReviewRepository(), deliveryClient, assignmentRepo)

		review, err := handler.Execute(context.Background(), commands.RateDriverCommand{
			DriverID: "driver-1", UserID: "user-1", DeliveryID: "delivery-1", Rating: 5, Comment: "good",
		})
		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}
		if review == nil {
			t.Fatal("Execute() returned nil review")
		}
	})

	t.Run("existing-review lookup error", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetDriver(baseDriver)
		deliveryClient := mocks.NewMockDeliveryServiceClient()
		deliveryClient.SetDelivery(delivery)
		reviewRepo := &reviewRepoWithErrors{MockReviewRepository: mocks.NewMockReviewRepository(), findDeliveryErr: errors.New("mongo down")}
		handler := commands.NewRateDriverHandler(driverRepo, reviewRepo, deliveryClient, mocks.NewMockAssignmentRepository())

		_, err := handler.Execute(context.Background(), commands.RateDriverCommand{
			DriverID: "driver-1", UserID: "user-1", DeliveryID: "delivery-1", Rating: 5,
		})
		if err == nil || err.Error() != "mongo down" {
			t.Errorf("Execute() error = %v, want mongo down", err)
		}
	})

	t.Run("review create error", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetDriver(baseDriver)
		deliveryClient := mocks.NewMockDeliveryServiceClient()
		deliveryClient.SetDelivery(delivery)
		reviewRepo := mocks.NewMockReviewRepository()
		// SetError only affects Create/Update paths; FindByDeliveryID ignores it
		reviewRepo.SetError(errors.New("insert failed"))
		handler := commands.NewRateDriverHandler(driverRepo, reviewRepo, deliveryClient, mocks.NewMockAssignmentRepository())

		_, err := handler.Execute(context.Background(), commands.RateDriverCommand{
			DriverID: "driver-1", UserID: "user-1", DeliveryID: "delivery-1", Rating: 5,
		})
		if err == nil || err.Error() != "insert failed" {
			t.Errorf("Execute() error = %v, want insert failed", err)
		}
	})

	t.Run("average rating calculation error still returns review", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetDriver(baseDriver)
		deliveryClient := mocks.NewMockDeliveryServiceClient()
		deliveryClient.SetDelivery(delivery)
		reviewRepo := &reviewRepoWithErrors{MockReviewRepository: mocks.NewMockReviewRepository(), avgErr: errors.New("agg failed")}
		handler := commands.NewRateDriverHandler(driverRepo, reviewRepo, deliveryClient, mocks.NewMockAssignmentRepository())

		review, err := handler.Execute(context.Background(), commands.RateDriverCommand{
			DriverID: "driver-1", UserID: "user-1", DeliveryID: "delivery-1", Rating: 5, Comment: "ok",
		})
		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}
		if review == nil {
			t.Fatal("Execute() returned nil review, want saved review despite rating calculation failure")
		}
	})
}
