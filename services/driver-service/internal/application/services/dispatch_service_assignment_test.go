package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/observability"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// ---------------------------------------------------------------------------
// AcceptAssignment
// ---------------------------------------------------------------------------

func TestAcceptAssignment(t *testing.T) {
	tests := []struct {
		name          string
		setupAssign   *domain.Assignment
		setupDriver   *domain.Driver
		findErr       error
		saveErr       error
		publishErr    error
		useMetrics    bool
		wantErr       error
		wantStatus    domain.AssignmentStatus
	}{
		{
			name:        "success - offer accepted",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered),
			setupDriver: newDriver("driver-1", domain.DriverStatusBusy),
			useMetrics:  true,
			wantErr:     nil,
			wantStatus:  domain.AssignmentStatusAccepted,
		},
		{
			name:        "success with nil metrics",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered),
			useMetrics:  false,
			wantErr:     nil,
			wantStatus:  domain.AssignmentStatusAccepted,
		},
		{
			name:        "find error",
			findErr:     errors.New("mongo down"),
			wantErr:     errors.New("mongo down"),
			wantStatus:  "",
		},
		{
			name:        "assignment not found",
			wantErr:     domain.ErrAssignmentNotFound,
			wantStatus:  "",
		},
		{
			name:        "wrong driver",
			setupAssign: newAssignment("a1", "driver-2", "delivery-1", domain.AssignmentStatusOffered),
			wantErr:     domain.ErrAssignmentInvalidState,
			wantStatus:  domain.AssignmentStatusOffered,
		},
		{
			name:        "idempotent when already accepted",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusAccepted),
			wantErr:     nil,
			wantStatus:  domain.AssignmentStatusAccepted,
		},
		{
			name:        "invalid state",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusRejected),
			wantErr:     domain.ErrAssignmentInvalidState,
			wantStatus:  domain.AssignmentStatusRejected,
		},
		{
			name:        "save error",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered),
			saveErr:     errors.New("save failed"),
			wantErr:     errors.New("save failed"),
			// the in-memory assignment is mutated before Save fails, so no status assertion
			wantStatus: "",
		},
		{
			name:        "publish failure is non-fatal",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered),
			publishErr:  errors.New("kafka down"),
			useMetrics:  true,
			wantErr:     nil,
			wantStatus:  domain.AssignmentStatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseAssignmentRepo := mocks.NewMockAssignmentRepository()
			var assignmentRepo ports.AssignmentRepository = baseAssignmentRepo
			if tt.setupAssign != nil {
				baseAssignmentRepo.SetAssignment(tt.setupAssign)
			}
			if tt.findErr != nil {
				baseAssignmentRepo.SetError(tt.findErr)
			}
			if tt.saveErr != nil {
				assignmentRepo = &assignmentRepoWithSaveError{MockAssignmentRepository: baseAssignmentRepo, saveErr: tt.saveErr}
			}
			eventPub := mocks.NewMockEventPublisher()
			if tt.publishErr != nil {
				eventPub.SetError("AssignmentAccepted", tt.publishErr)
			}
			var metrics *observability.Metrics
			if tt.useMetrics {
				metrics = sharedDispatchMetrics
			}

			svc := services.NewDispatchService(mocks.NewMockDriverRepository(), assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), eventPub, nil, metrics)

			err := svc.AcceptAssignment(context.Background(), "a1", "driver-1")

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("AcceptAssignment() expected error %v, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("AcceptAssignment() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("AcceptAssignment() unexpected error: %v", err)
			}

			if tt.wantStatus != "" {
				assignment, findErr := baseAssignmentRepo.FindByID(context.Background(), "a1")
				if findErr != nil {
					t.Fatalf("FindByID() unexpected error: %v", findErr)
				}
				if assignment == nil {
					t.Fatal("assignment not found in repo")
				}
				if assignment.Status != tt.wantStatus {
					t.Errorf("assignment status = %s, want %s", assignment.Status, tt.wantStatus)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RejectAssignment
// ---------------------------------------------------------------------------

func TestRejectAssignment(t *testing.T) {
	tests := []struct {
		name         string
		setupAssign  *domain.Assignment
		setupDriver  *domain.Driver
		findErr      error
		saveErr      error
		driverFindErr error
		publishErr   error
		useMetrics   bool
		wantErr      error
		wantStatus   domain.AssignmentStatus
		wantDriver   domain.DriverStatus
	}{
		{
			name:        "success - offer rejected and driver released",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered),
			setupDriver: newDriver("driver-1", domain.DriverStatusBusy),
			useMetrics:  true,
			wantErr:     nil,
			wantStatus:  domain.AssignmentStatusRejected,
			wantDriver:  domain.DriverStatusAvailable,
		},
		{
			name:        "success with nil metrics",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered),
			setupDriver: newDriver("driver-1", domain.DriverStatusBusy),
			wantErr:     nil,
			wantStatus:  domain.AssignmentStatusRejected,
			wantDriver:  domain.DriverStatusAvailable,
		},
		{
			name:        "driver lookup error is tolerated",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered),
			driverFindErr: errors.New("mongo down"),
			useMetrics:  true,
			wantErr:     nil,
			wantStatus:  domain.AssignmentStatusRejected,
		},
		{
			name:        "driver missing from repo is tolerated",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered),
			wantErr:     nil,
			wantStatus:  domain.AssignmentStatusRejected,
		},
		{
			name:        "find error",
			findErr:     errors.New("mongo down"),
			wantErr:     errors.New("mongo down"),
		},
		{
			name:        "assignment not found",
			wantErr:     domain.ErrAssignmentNotFound,
		},
		{
			name:        "wrong driver",
			setupAssign: newAssignment("a1", "driver-2", "delivery-1", domain.AssignmentStatusOffered),
			wantErr:     domain.ErrAssignmentInvalidState,
			wantStatus:  domain.AssignmentStatusOffered,
		},
		{
			name:        "idempotent when already rejected",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusRejected),
			wantErr:     nil,
			wantStatus:  domain.AssignmentStatusRejected,
		},
		{
			name:        "invalid state",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusAccepted),
			wantErr:     domain.ErrAssignmentInvalidState,
			wantStatus:  domain.AssignmentStatusAccepted,
		},
		{
			name:        "save error",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered),
			saveErr:     errors.New("save failed"),
			wantErr:     errors.New("save failed"),
			// the in-memory assignment is mutated before Save fails, so no status assertion
			wantStatus: "",
		},
		{
			name:        "publish failure is non-fatal",
			setupAssign: newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered),
			setupDriver: newDriver("driver-1", domain.DriverStatusBusy),
			publishErr:  errors.New("kafka down"),
			useMetrics:  true,
			wantErr:     nil,
			wantStatus:  domain.AssignmentStatusRejected,
			wantDriver:  domain.DriverStatusAvailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseAssignmentRepo := mocks.NewMockAssignmentRepository()
			var assignmentRepo ports.AssignmentRepository = baseAssignmentRepo
			if tt.setupAssign != nil {
				baseAssignmentRepo.SetAssignment(tt.setupAssign)
			}
			if tt.findErr != nil {
				baseAssignmentRepo.SetError(tt.findErr)
			}
			if tt.saveErr != nil {
				assignmentRepo = &assignmentRepoWithSaveError{MockAssignmentRepository: baseAssignmentRepo, saveErr: tt.saveErr}
			}
			baseDriverRepo := mocks.NewMockDriverRepository()
			if tt.setupDriver != nil {
				baseDriverRepo.SetDriver(tt.setupDriver)
			}
			if tt.driverFindErr != nil {
				baseDriverRepo.SetError(tt.driverFindErr)
			}
			eventPub := mocks.NewMockEventPublisher()
			if tt.publishErr != nil {
				eventPub.SetError("AssignmentRejected", tt.publishErr)
			}
			var metrics *observability.Metrics
			if tt.useMetrics {
				metrics = sharedDispatchMetrics
			}

			svc := services.NewDispatchService(baseDriverRepo, assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), eventPub, nil, metrics)

			err := svc.RejectAssignment(context.Background(), "a1", "driver-1", "driver busy")

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("RejectAssignment() expected error %v, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("RejectAssignment() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("RejectAssignment() unexpected error: %v", err)
			}

			if tt.wantStatus != "" {
				assignment, findErr := baseAssignmentRepo.FindByID(context.Background(), "a1")
				if findErr != nil {
					t.Fatalf("FindByID() unexpected error: %v", findErr)
				}
				if assignment == nil {
					t.Fatal("assignment not found in repo")
				}
				if assignment.Status != tt.wantStatus {
					t.Errorf("assignment status = %s, want %s", assignment.Status, tt.wantStatus)
				}
			}
			if tt.wantDriver != "" {
				if tt.driverFindErr == nil {
					driver, _ := baseDriverRepo.FindByID(context.Background(), "driver-1")
					if driver == nil || driver.Status != tt.wantDriver {
						t.Errorf("driver = %+v, want status %s", driver, tt.wantDriver)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ReleaseDriverByAssignment
// ---------------------------------------------------------------------------

func TestReleaseDriverByAssignment(t *testing.T) {
	t.Run("find error", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetError(errors.New("mongo down"))
		svc := services.NewDispatchService(mocks.NewMockDriverRepository(), assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), mocks.NewMockEventPublisher(), nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriverByAssignment(context.Background(), "a1"); err == nil {
			t.Error("ReleaseDriverByAssignment() expected error, got nil")
		}
	})

	t.Run("assignment not found", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		svc := services.NewDispatchService(mocks.NewMockDriverRepository(), assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), mocks.NewMockEventPublisher(), nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriverByAssignment(context.Background(), "a1"); err == nil || err.Error() != domain.ErrAssignmentNotFound.Error() {
			t.Errorf("ReleaseDriverByAssignment() error = %v, want ErrAssignmentNotFound", err)
		}
	})

	for _, status := range []domain.AssignmentStatus{domain.AssignmentStatusActive, domain.AssignmentStatusAccepted} {
		t.Run("non-terminal status "+string(status)+" completes assignment", func(t *testing.T) {
			assignmentRepo := mocks.NewMockAssignmentRepository()
			assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", status))
			baseDriverRepo := mocks.NewMockDriverRepository()
			baseDriverRepo.SetDriver(newDriver("driver-1", domain.DriverStatusBusy))
			eventPub := mocks.NewMockEventPublisher()

			svc := services.NewDispatchService(baseDriverRepo, assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), eventPub, nil, sharedDispatchMetrics)

			if err := svc.ReleaseDriverByAssignment(context.Background(), "a1"); err != nil {
				t.Fatalf("ReleaseDriverByAssignment() unexpected error: %v", err)
			}
			assignment, _ := assignmentRepo.FindByID(context.Background(), "a1")
			if assignment == nil || assignment.Status != domain.AssignmentStatusCompleted {
				t.Errorf("assignment = %+v, want COMPLETED", assignment)
			}
			driver, _ := baseDriverRepo.FindByID(context.Background(), "driver-1")
			if driver == nil || driver.Status != domain.DriverStatusAvailable {
				t.Errorf("driver = %+v, want AVAILABLE", driver)
			}
			events := eventPub.GetEvents()
			if !contains(events, "DriverAvailable:driver-1") {
				t.Errorf("events = %v, want DriverAvailable:driver-1", events)
			}
			if !contains(events, "AssignmentReleased:a1") {
				t.Errorf("events = %v, want AssignmentReleased:a1", events)
			}
		})
	}

	t.Run("offered assignment expires", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered))
		baseDriverRepo := mocks.NewMockDriverRepository()
		baseDriverRepo.SetDriver(newDriver("driver-1", domain.DriverStatusBusy))
		eventPub := mocks.NewMockEventPublisher()

		svc := services.NewDispatchService(baseDriverRepo, assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), eventPub, nil, sharedDispatchMetrics)

		if err := svc.ReleaseDriverByAssignment(context.Background(), "a1"); err != nil {
			t.Fatalf("ReleaseDriverByAssignment() unexpected error: %v", err)
		}
		assignment, _ := assignmentRepo.FindByID(context.Background(), "a1")
		if assignment == nil || assignment.Status != domain.AssignmentStatusExpired {
			t.Errorf("assignment = %+v, want EXPIRED", assignment)
		}
		events := eventPub.GetEvents()
		if !contains(events, "AssignmentExpired:a1") {
			t.Errorf("events = %v, want AssignmentExpired:a1", events)
		}
		if !contains(events, "DriverAvailable:driver-1") {
			t.Errorf("events = %v, want DriverAvailable:driver-1", events)
		}
	})

	t.Run("terminal assignment is idempotent", func(t *testing.T) {
		for _, status := range []domain.AssignmentStatus{
			domain.AssignmentStatusCompleted,
			domain.AssignmentStatusRejected,
			domain.AssignmentStatusExpired,
			domain.AssignmentStatusCancelled,
		} {
			assignmentRepo := mocks.NewMockAssignmentRepository()
			assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", status))
			eventPub := mocks.NewMockEventPublisher()
			svc := services.NewDispatchService(mocks.NewMockDriverRepository(), assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), eventPub, nil, sharedDispatchMetrics)

			if err := svc.ReleaseDriverByAssignment(context.Background(), "a1"); err != nil {
				t.Errorf("status %s: ReleaseDriverByAssignment() unexpected error: %v", status, err)
			}
			if len(eventPub.GetEvents()) != 0 {
				t.Errorf("status %s: events = %v, want none", status, eventPub.GetEvents())
			}
		}
	})

	t.Run("assignment save error", func(t *testing.T) {
		baseAssignmentRepo := mocks.NewMockAssignmentRepository()
		baseAssignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered))
		assignmentRepo := &assignmentRepoWithSaveError{MockAssignmentRepository: baseAssignmentRepo, saveErr: errors.New("save failed")}
		svc := services.NewDispatchService(mocks.NewMockDriverRepository(), assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), mocks.NewMockEventPublisher(), nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriverByAssignment(context.Background(), "a1"); err == nil {
			t.Error("ReleaseDriverByAssignment() expected error, got nil")
		}
	})

	t.Run("driver missing skips release", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered))
		eventPub := mocks.NewMockEventPublisher()
		svc := services.NewDispatchService(mocks.NewMockDriverRepository(), assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), eventPub, nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriverByAssignment(context.Background(), "a1"); err != nil {
			t.Fatalf("ReleaseDriverByAssignment() unexpected error: %v", err)
		}
		if !contains(eventPub.GetEvents(), "AssignmentExpired:a1") {
			t.Errorf("events = %v, want AssignmentExpired:a1", eventPub.GetEvents())
		}
	})

	t.Run("driver save error", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusActive))
		baseDriverRepo := mocks.NewMockDriverRepository()
		baseDriverRepo.SetDriver(newDriver("driver-1", domain.DriverStatusBusy))
		driverRepo := &driverRepoWithSaveError{MockDriverRepository: baseDriverRepo, saveErr: errors.New("save failed")}
		svc := services.NewDispatchService(driverRepo, assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), mocks.NewMockEventPublisher(), nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriverByAssignment(context.Background(), "a1"); err == nil {
			t.Error("ReleaseDriverByAssignment() expected error, got nil")
		}
	})

	t.Run("driver available publish failure is non-fatal", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusActive))
		baseDriverRepo := mocks.NewMockDriverRepository()
		baseDriverRepo.SetDriver(newDriver("driver-1", domain.DriverStatusBusy))
		eventPub := mocks.NewMockEventPublisher()
		eventPub.SetError("DriverAvailable", errors.New("kafka down"))
		svc := services.NewDispatchService(baseDriverRepo, assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), eventPub, nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriverByAssignment(context.Background(), "a1"); err != nil {
			t.Fatalf("ReleaseDriverByAssignment() unexpected error: %v", err)
		}
	})

	t.Run("expired publish failure is non-fatal", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusOffered))
		baseDriverRepo := mocks.NewMockDriverRepository()
		baseDriverRepo.SetDriver(newDriver("driver-1", domain.DriverStatusBusy))
		eventPub := mocks.NewMockEventPublisher()
		eventPub.SetError("AssignmentExpired", errors.New("kafka down"))
		svc := services.NewDispatchService(baseDriverRepo, assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), eventPub, nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriverByAssignment(context.Background(), "a1"); err != nil {
			t.Fatalf("ReleaseDriverByAssignment() unexpected error: %v", err)
		}
	})

	t.Run("released publish failure is non-fatal", func(t *testing.T) {
		assignmentRepo := mocks.NewMockAssignmentRepository()
		assignmentRepo.SetAssignment(newAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusActive))
		baseDriverRepo := mocks.NewMockDriverRepository()
		baseDriverRepo.SetDriver(newDriver("driver-1", domain.DriverStatusBusy))
		eventPub := mocks.NewMockEventPublisher()
		eventPub.SetError("AssignmentReleased", errors.New("kafka down"))
		svc := services.NewDispatchService(baseDriverRepo, assignmentRepo, mocks.NewMockLocationStore(), mocks.NewMockLockManager(), eventPub, nil, sharedDispatchMetrics)
		if err := svc.ReleaseDriverByAssignment(context.Background(), "a1"); err != nil {
			t.Fatalf("ReleaseDriverByAssignment() unexpected error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// ValidateDriverID / ValidateUserID
// ---------------------------------------------------------------------------

func TestValidateDriverID(t *testing.T) {
	driverRepo := mocks.NewMockDriverRepository()
	driverRepo.SetDriver(newDriver("driver-1", domain.DriverStatusAvailable))
	svc := services.NewDispatchService(driverRepo, mocks.NewMockAssignmentRepository(), mocks.NewMockLocationStore(), mocks.NewMockLockManager(), mocks.NewMockEventPublisher(), nil, sharedDispatchMetrics)

	if ok, status := svc.ValidateDriverID(context.Background(), "driver-1"); !ok || status != string(domain.DriverStatusAvailable) {
		t.Errorf("ValidateDriverID() = (%v, %q), want (true, %q)", ok, status, domain.DriverStatusAvailable)
	}
	if ok, status := svc.ValidateDriverID(context.Background(), "missing"); ok || status != "" {
		t.Errorf("ValidateDriverID(missing) = (%v, %q), want (false, \"\")", ok, status)
	}

	driverRepo.SetError(errors.New("mongo down"))
	if ok, status := svc.ValidateDriverID(context.Background(), "driver-1"); ok || status != "" {
		t.Errorf("ValidateDriverID(error) = (%v, %q), want (false, \"\")", ok, status)
	}
}

func TestValidateUserID(t *testing.T) {
	driverRepo := mocks.NewMockDriverRepository()
	driverRepo.SetDriver(newDriver("driver-1", domain.DriverStatusBusy))
	svc := services.NewDispatchService(driverRepo, mocks.NewMockAssignmentRepository(), mocks.NewMockLocationStore(), mocks.NewMockLockManager(), mocks.NewMockEventPublisher(), nil, sharedDispatchMetrics)

	if ok, status := svc.ValidateUserID(context.Background(), "user-driver-1"); !ok || status != string(domain.DriverStatusBusy) {
		t.Errorf("ValidateUserID() = (%v, %q), want (true, %q)", ok, status, domain.DriverStatusBusy)
	}
	if ok, status := svc.ValidateUserID(context.Background(), "missing"); ok || status != "" {
		t.Errorf("ValidateUserID(missing) = (%v, %q), want (false, \"\")", ok, status)
	}

	driverRepo.SetError(errors.New("mongo down"))
	if ok, status := svc.ValidateUserID(context.Background(), "user-driver-1"); ok || status != "" {
		t.Errorf("ValidateUserID(error) = (%v, %q), want (false, \"\")", ok, status)
	}
}
