package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/observability"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// observability.NewMetrics registers collectors on the default Prometheus
// registry, which panics when called twice. Create it exactly once for this
// test binary and share it between tests (counters simply keep incrementing).
var sharedDispatchMetrics = observability.NewMetrics()

// ---------------------------------------------------------------------------
// Local test doubles (extra fakes live in _test.go, mocks.go is never touched)
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

type assignmentRepoWithFindActiveError struct {
	*mocks.MockAssignmentRepository
	findErr error
}

func (r *assignmentRepoWithFindActiveError) FindActiveByDriver(ctx context.Context, driverID string) (*domain.Assignment, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.MockAssignmentRepository.FindActiveByDriver(ctx, driverID)
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

type locationStoreWithSearchError struct {
	*mocks.MockLocationStore
	searchErr error
}

func (r *locationStoreWithSearchError) NearbyDriversSorted(ctx context.Context, longitude, latitude, radiusKm float64) ([]ports.GeoDriverResult, error) {
	if r.searchErr != nil {
		return nil, r.searchErr
	}
	return r.MockLocationStore.NearbyDriversSorted(ctx, longitude, latitude, radiusKm)
}

func newDriver(id string, status domain.DriverStatus) *domain.Driver {
	return &domain.Driver{
		ID:        id,
		UserID:    "user-" + id,
		Status:    status,
		IsActive:  true,
		IsBlocked: false,
		Vehicle:   domain.VehicleInfo{Type: domain.VehicleTypeCar},
	}
}

func newAssignment(id, driverID, deliveryID string, status domain.AssignmentStatus) *domain.Assignment {
	now := time.Now()
	return &domain.Assignment{
		ID:          id,
		DriverID:    driverID,
		DeliveryID:  deliveryID,
		Status:      status,
		OfferedAt:   now,
		ExpiresAt:   now.Add(30 * time.Second),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func contains(events []string, want string) bool {
	for _, e := range events {
		if e == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// FindAvailableDrivers
// ---------------------------------------------------------------------------

func TestFindAvailableDrivers_GeoPath(t *testing.T) {
	driverRepo := mocks.NewMockDriverRepository()
	locationStore := mocks.NewMockLocationStore()
	assignmentRepo := mocks.NewMockAssignmentRepository()
	eventPub := mocks.NewMockEventPublisher()
	lockManager := mocks.NewMockLockManager()

	// geo store knows about four drivers, sorted by distance
	locationStore.SetLocation("d1", 31.0, 30.0, 100)
	locationStore.SetLocation("d2", 31.0, 30.0, 200)
	locationStore.SetLocation("d3", 31.0, 30.0, 50)
	locationStore.SetLocation("d4", 31.0, 30.0, 300)

	driverRepo.SetDriver(newDriver("d1", domain.DriverStatusAvailable))
	excluded := newDriver("d2", domain.DriverStatusAvailable)
	driverRepo.SetDriver(excluded)
	driverRepo.SetDriver(newDriver("d3", domain.DriverStatusOffline))
	wrongVehicle := newDriver("d4", domain.DriverStatusAvailable)
	wrongVehicle.Vehicle.Type = domain.VehicleTypeMotorcycle
	driverRepo.SetDriver(wrongVehicle)

	svc := services.NewDispatchService(driverRepo, assignmentRepo, locationStore, lockManager, eventPub, domain.NewDispatchPolicy(), sharedDispatchMetrics)

	candidates, err := svc.FindAvailableDrivers(context.Background(), 30.0, 31.0, 5.0, domain.VehicleTypeCar, "delivery-1", "d2")
	if err != nil {
		t.Fatalf("FindAvailableDrivers() unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("FindAvailableDrivers() returned %d candidates, want 1", len(candidates))
	}
	if candidates[0].DriverID != "d1" {
		t.Errorf("candidate DriverID = %s, want d1", candidates[0].DriverID)
	}
	if candidates[0].DistanceMeters != 100 || candidates[0].RankingScore != 100 {
		t.Errorf("candidate distance/rank = %v/%v, want 100/100", candidates[0].DistanceMeters, candidates[0].RankingScore)
	}
}

func TestFindAvailableDrivers_RadiusDefaultAndNilPolicy(t *testing.T) {
	driverRepo := mocks.NewMockDriverRepository()
	locationStore := mocks.NewMockLocationStore()
	assignmentRepo := mocks.NewMockAssignmentRepository()
	eventPub := mocks.NewMockEventPublisher()
	lockManager := mocks.NewMockLockManager()

	driverRepo.SetDriver(newDriver("d1", domain.DriverStatusAvailable))

	// nil dispatchPolicy exercises the default-policy branch, radius <= 0 the default radius branch
	svc := services.NewDispatchService(driverRepo, assignmentRepo, locationStore, lockManager, eventPub, nil, nil)

	candidates, err := svc.FindAvailableDrivers(context.Background(), 30.0, 31.0, 0, "", "delivery-1")
	if err != nil {
		t.Fatalf("FindAvailableDrivers() unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("FindAvailableDrivers() returned %d candidates, want 1 (mongo fallback)", len(candidates))
	}
	if candidates[0].DriverID != "d1" {
		t.Errorf("candidate DriverID = %s, want d1", candidates[0].DriverID)
	}
	if candidates[0].RankingScore != 999999 {
		t.Errorf("fallback RankingScore = %v, want 999999", candidates[0].RankingScore)
	}
}

func TestFindAvailableDrivers_AllGeoExcludedFallsBackToMongo(t *testing.T) {
	driverRepo := mocks.NewMockDriverRepository()
	locationStore := mocks.NewMockLocationStore()
	assignmentRepo := mocks.NewMockAssignmentRepository()
	eventPub := mocks.NewMockEventPublisher()
	lockManager := mocks.NewMockLockManager()

	locationStore.SetLocation("d1", 31.0, 30.0, 100)
	driverRepo.SetDriver(newDriver("d1", domain.DriverStatusAvailable))
	driverRepo.SetDriver(newDriver("d2", domain.DriverStatusAvailable))

	svc := services.NewDispatchService(driverRepo, assignmentRepo, locationStore, lockManager, eventPub, domain.NewDispatchPolicy(), sharedDispatchMetrics)

	// d1 is in the geo store but excluded → candidateIDs empty → mongo fallback
	candidates, err := svc.FindAvailableDrivers(context.Background(), 30.0, 31.0, 5.0, domain.VehicleTypeCar, "delivery-1", "d1")
	if err != nil {
		t.Fatalf("FindAvailableDrivers() unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("FindAvailableDrivers() returned %d candidates, want 1 (d2 via fallback)", len(candidates))
	}
	if candidates[0].DriverID != "d2" {
		t.Errorf("candidate DriverID = %s, want d2", candidates[0].DriverID)
	}
	if candidates[0].DistanceMeters != 0 || candidates[0].RankingScore != 999999 {
		t.Errorf("fallback candidate distance/rank = %v/%v, want 0/999999", candidates[0].DistanceMeters, candidates[0].RankingScore)
	}
}

func TestFindAvailableDrivers_GeoSearchErrorFallsBackToMongo(t *testing.T) {
	driverRepo := mocks.NewMockDriverRepository()
	baseStore := mocks.NewMockLocationStore()
	locationStore := &locationStoreWithSearchError{MockLocationStore: baseStore, searchErr: errors.New("redis down")}
	assignmentRepo := mocks.NewMockAssignmentRepository()
	eventPub := mocks.NewMockEventPublisher()
	lockManager := mocks.NewMockLockManager()

	driverRepo.SetDriver(newDriver("d1", domain.DriverStatusAvailable))
	driverRepo.SetDriver(newDriver("excluded", domain.DriverStatusAvailable))

	svc := services.NewDispatchService(driverRepo, assignmentRepo, locationStore, lockManager, eventPub, domain.NewDispatchPolicy(), sharedDispatchMetrics)

	candidates, err := svc.FindAvailableDrivers(context.Background(), 30.0, 31.0, 5.0, domain.VehicleTypeCar, "delivery-1", "excluded")
	if err != nil {
		t.Fatalf("FindAvailableDrivers() unexpected error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].DriverID != "d1" {
		t.Fatalf("FindAvailableDrivers() = %v, want single candidate d1", candidates)
	}
}

func TestFindAvailableDrivers_RepositoryErrors(t *testing.T) {
	t.Run("FindByIDs error is returned", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		locationStore := mocks.NewMockLocationStore()
		assignmentRepo := mocks.NewMockAssignmentRepository()
		eventPub := mocks.NewMockEventPublisher()
		lockManager := mocks.NewMockLockManager()

		locationStore.SetLocation("d1", 31.0, 30.0, 100)
		driverRepo.SetError(errors.New("mongo down"))

		svc := services.NewDispatchService(driverRepo, assignmentRepo, locationStore, lockManager, eventPub, domain.NewDispatchPolicy(), sharedDispatchMetrics)

		if _, err := svc.FindAvailableDrivers(context.Background(), 30.0, 31.0, 5.0, domain.VehicleTypeCar, "delivery-1"); err == nil {
			t.Fatal("FindAvailableDrivers() expected error, got nil")
		}
	})

	t.Run("mongo fallback error is returned", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		locationStore := mocks.NewMockLocationStore()
		assignmentRepo := mocks.NewMockAssignmentRepository()
		eventPub := mocks.NewMockEventPublisher()
		lockManager := mocks.NewMockLockManager()

		driverRepo.SetError(errors.New("mongo down"))

		svc := services.NewDispatchService(driverRepo, assignmentRepo, locationStore, lockManager, eventPub, domain.NewDispatchPolicy(), sharedDispatchMetrics)

		if _, err := svc.FindAvailableDrivers(context.Background(), 30.0, 31.0, 5.0, domain.VehicleTypeCar, "delivery-1"); err == nil {
			t.Fatal("FindAvailableDrivers() expected error, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// ReserveDriver
// ---------------------------------------------------------------------------

func TestReserveDriver(t *testing.T) {
	errBoom := errors.New("lock backend down")

	tests := []struct {
		name           string
		setupDriver    *domain.Driver
		holdLock       bool
		lockErr        error
		driverRepoErr  error
		driverSaveErr  error
		assignSaveErr  error
		publishErr     error
		useMetrics     bool
		wantReserved   bool
		wantErr        error
		wantEvents     string
	}{
		{
			name:         "success - driver reserved and assignment offered",
			setupDriver:  newDriver("driver-1", domain.DriverStatusAvailable),
			useMetrics:   true,
			wantReserved: true,
			wantEvents:   "AssignmentOffered:delivery-1-driver-1",
		},
		{
			name:         "lock already held - ErrDriverAlreadyReserved",
			setupDriver:  newDriver("driver-1", domain.DriverStatusAvailable),
			holdLock:     true,
			useMetrics:   true,
			wantReserved: false,
			wantErr:      domain.ErrDriverAlreadyReserved,
		},
		{
			name:         "lock acquire error",
			setupDriver:  newDriver("driver-1", domain.DriverStatusAvailable),
			lockErr:      errBoom,
			useMetrics:   true,
			wantReserved: false,
			wantErr:      errBoom,
		},
		{
			name:         "driver not found",
			useMetrics:   true,
			wantReserved: false,
			wantErr:      domain.ErrDriverNotFound,
		},
		{
			name:         "driver repo find error",
			setupDriver:  newDriver("driver-1", domain.DriverStatusAvailable),
			driverRepoErr: errors.New("mongo down"),
			useMetrics:   true,
			wantReserved: false,
			wantErr:      errors.New("mongo down"),
		},
		{
			name:         "driver not available",
			setupDriver:  newDriver("driver-1", domain.DriverStatusOffline),
			useMetrics:   true,
			wantReserved: false,
			wantErr:      domain.ErrDriverNotAvailableForAssignment,
		},
		{
			name: "driver.Reserve() transition error (blocked)",
			setupDriver: func() *domain.Driver {
				d := newDriver("driver-1", domain.DriverStatusAvailable)
				d.IsBlocked = true
				return d
			}(),
			useMetrics:   true,
			wantReserved: false,
			wantErr:      domain.ErrDriverBlocked,
		},
		{
			name:         "driver save error",
			setupDriver:  newDriver("driver-1", domain.DriverStatusAvailable),
			driverSaveErr: errors.New("save failed"),
			useMetrics:   true,
			wantReserved: false,
			wantErr:      errors.New("save failed"),
		},
		{
			name:         "assignment save error rolls driver back",
			setupDriver:  newDriver("driver-1", domain.DriverStatusAvailable),
			assignSaveErr: errors.New("assign save failed"),
			useMetrics:   true,
			wantReserved: false,
			wantErr:      errors.New("assign save failed"),
		},
		{
			name:         "publish failure is non-fatal",
			setupDriver:  newDriver("driver-1", domain.DriverStatusAvailable),
			publishErr:   errors.New("kafka down"),
			useMetrics:   true,
			wantReserved: true,
			wantEvents:   "AssignmentOffered:delivery-1-driver-1",
		},
		{
			name:         "success with nil metrics",
			setupDriver:  newDriver("driver-1", domain.DriverStatusAvailable),
			useMetrics:   false,
			wantReserved: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var driverRepo ports.DriverRepository = mocks.NewMockDriverRepository()
			baseDriverRepo := driverRepo.(*mocks.MockDriverRepository)
			var assignmentRepo ports.AssignmentRepository = mocks.NewMockAssignmentRepository()
			baseAssignmentRepo := assignmentRepo.(*mocks.MockAssignmentRepository)
			lockManager := mocks.NewMockLockManager()
			eventPub := mocks.NewMockEventPublisher()
			locationStore := mocks.NewMockLocationStore()

			if tt.setupDriver != nil {
				baseDriverRepo.SetDriver(tt.setupDriver)
			}
			if tt.driverRepoErr != nil {
				baseDriverRepo.SetError(tt.driverRepoErr)
			}
			if tt.driverSaveErr != nil {
				driverRepo = &driverRepoWithSaveError{MockDriverRepository: baseDriverRepo, saveErr: tt.driverSaveErr}
			}
			if tt.assignSaveErr != nil {
				assignmentRepo = &assignmentRepoWithSaveError{MockAssignmentRepository: baseAssignmentRepo, saveErr: tt.assignSaveErr}
			}
			var lock ports.LockManager = lockManager
			if tt.lockErr != nil {
				lock = &lockManagerWithAcquireError{MockLockManager: mocks.NewMockLockManager(), acquireErr: tt.lockErr}
			}
			if tt.holdLock {
				if _, err := lock.Acquire(context.Background(), "driver-1", 30*time.Second); err != nil {
					t.Fatalf("pre-acquire lock failed: %v", err)
				}
			}
			if tt.publishErr != nil {
				eventPub.SetError("AssignmentOffered", tt.publishErr)
			}

			var metrics *observability.Metrics
			if tt.useMetrics {
				metrics = sharedDispatchMetrics
			}

			svc := services.NewDispatchService(driverRepo, assignmentRepo, locationStore, lock, eventPub, domain.NewDispatchPolicy(), metrics)

			reserved, err := svc.ReserveDriver(context.Background(), "driver-1", "delivery-1")

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("ReserveDriver() expected error %v, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("ReserveDriver() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("ReserveDriver() unexpected error: %v", err)
			}
			if reserved != tt.wantReserved {
				t.Errorf("ReserveDriver() reserved = %v, want %v", reserved, tt.wantReserved)
			}

			if tt.wantReserved {
				driver, findErr := baseDriverRepo.FindByID(context.Background(), "driver-1")
				if findErr != nil {
					t.Fatalf("FindByID() unexpected error: %v", findErr)
				}
				if driver.Status != domain.DriverStatusBusy {
					t.Errorf("driver status = %s, want BUSY", driver.Status)
				}
				assignment, findErr := baseAssignmentRepo.FindByID(context.Background(), "delivery-1-driver-1")
				if findErr != nil {
					t.Fatalf("assignment FindByID() unexpected error: %v", findErr)
				}
				if assignment == nil || assignment.Status != domain.AssignmentStatusOffered {
					t.Errorf("assignment = %+v, want status OFFERED", assignment)
				}
			}

			// Rollback: assignment save failure must restore the driver to AVAILABLE
			if tt.assignSaveErr != nil {
				driver, _ := baseDriverRepo.FindByID(context.Background(), "driver-1")
				if driver == nil || driver.Status != domain.DriverStatusAvailable {
					t.Errorf("driver after rollback = %+v, want AVAILABLE", driver)
				}
			}

			if tt.wantEvents != "" {
				if !contains(eventPub.GetEvents(), tt.wantEvents) {
					t.Errorf("published events = %v, want to contain %s", eventPub.GetEvents(), tt.wantEvents)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ReleaseDriver
// ---------------------------------------------------------------------------

func TestReleaseDriver(t *testing.T) {
	t.Run("no active assignment", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		svc := services.NewDispatchService(mocks.NewMockDriverRepository(), assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), mocks.NewMockEventPublisher(), nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriver(context.Background(), "driver-1", "delivery-1"); !errors.Is(err, domain.ErrAssignmentNotFound) && err != domain.ErrAssignmentNotFound {
			t.Errorf("ReleaseDriver() error = %v, want ErrAssignmentNotFound", err)
		}
	})

	t.Run("FindActiveByDriver error", func(t *testing.T) {
		base := mocks.NewMockAssignmentRepository()
		assignmentRepo := &assignmentRepoWithFindActiveError{MockAssignmentRepository: base, findErr: errors.New("mongo down")}
		svc := services.NewDispatchService(mocks.NewMockDriverRepository(), assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), mocks.NewMockEventPublisher(), nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriver(context.Background(), "driver-1", "delivery-1"); err == nil {
			t.Error("ReleaseDriver() expected error, got nil")
		}
	})

	t.Run("UpdateStatus error", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusActive))
		assignmentRepo.SetError(errors.New("mongo down"))
		svc := services.NewDispatchService(mocks.NewMockDriverRepository(), assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), mocks.NewMockEventPublisher(), nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriver(context.Background(), "driver-1", "delivery-1"); err == nil {
			t.Error("ReleaseDriver() expected error, got nil")
		}
	})

	t.Run("driver find error", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusActive))
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetError(errors.New("mongo down"))
		svc := services.NewDispatchService(driverRepo, assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), mocks.NewMockEventPublisher(), nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriver(context.Background(), "driver-1", "delivery-1"); err == nil {
			t.Error("ReleaseDriver() expected error, got nil")
		}
	})

	t.Run("driver missing from repo still succeeds", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusActive))
		eventPub := mocks.NewMockEventPublisher()
		svc := services.NewDispatchService(mocks.NewMockDriverRepository(), assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), eventPub, nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriver(context.Background(), "driver-1", "delivery-1"); err != nil {
			t.Fatalf("ReleaseDriver() unexpected error: %v", err)
		}
		if !contains(eventPub.GetEvents(), "DriverAvailable:driver-1") {
			t.Errorf("events = %v, want DriverAvailable:driver-1", eventPub.GetEvents())
		}
	})

	t.Run("driver save error", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusActive))
		baseDriverRepo := mocks.NewMockDriverRepository()
		baseDriverRepo.SetDriver(newDriver("driver-1", domain.DriverStatusBusy))
		driverRepo := &driverRepoWithSaveError{MockDriverRepository: baseDriverRepo, saveErr: errors.New("save failed")}
		svc := services.NewDispatchService(driverRepo, assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), mocks.NewMockEventPublisher(), nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriver(context.Background(), "driver-1", "delivery-1"); err == nil {
			t.Error("ReleaseDriver() expected error, got nil")
		}
	})

	t.Run("publish failure is non-fatal", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusActive))
		baseDriverRepo := mocks.NewMockDriverRepository()
		baseDriverRepo.SetDriver(newDriver("driver-1", domain.DriverStatusBusy))
		eventPub := mocks.NewMockEventPublisher()
		eventPub.SetError("DriverAvailable", errors.New("kafka down"))
		svc := services.NewDispatchService(baseDriverRepo, assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), eventPub, nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriver(context.Background(), "driver-1", "delivery-1"); err != nil {
			t.Fatalf("ReleaseDriver() unexpected error: %v", err)
		}
	})

	t.Run("success releases driver to AVAILABLE", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusActive))
		baseDriverRepo := mocks.NewMockDriverRepository()
		baseDriverRepo.SetDriver(newDriver("driver-1", domain.DriverStatusBusy))
		svc := services.NewDispatchService(baseDriverRepo, assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), mocks.NewMockEventPublisher(), nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriver(context.Background(), "driver-1", "delivery-1"); err != nil {
			t.Fatalf("ReleaseDriver() unexpected error: %v", err)
		}
		driver, _ := baseDriverRepo.FindByID(context.Background(), "driver-1")
		if driver == nil || driver.Status != domain.DriverStatusAvailable {
			t.Errorf("driver = %+v, want AVAILABLE", driver)
		}
		assignment, _ := assignmentRepo.FindByID(context.Background(), "a1")
		if assignment == nil || assignment.Status != domain.AssignmentStatusCompleted {
			t.Errorf("assignment = %+v, want COMPLETED", assignment)
		}
	})
}
