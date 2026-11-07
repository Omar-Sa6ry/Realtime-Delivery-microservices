package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

// TestIsValidAssignmentTransitionUnknownSource covers the branch where the
// source status has no entry in the transition map (COMPLETED is absent,
// as are arbitrary garbage values).
func TestIsValidAssignmentTransitionUnknownSource(t *testing.T) {
	tests := []struct {
		name string
		from domain.AssignmentStatus
		to   domain.AssignmentStatus
	}{
		{"completed has no outgoing transitions", domain.AssignmentStatusCompleted, domain.AssignmentStatusActive},
		{"garbage source status", domain.AssignmentStatus("BOGUS"), domain.AssignmentStatusOffered},
		{"empty source status", domain.AssignmentStatus(""), domain.AssignmentStatusOffered},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if domain.IsValidAssignmentTransition(tt.from, tt.to, "anything") {
				t.Errorf("IsValidAssignmentTransition(%s -> %s) = true, want false", tt.from, tt.to)
			}
		})
	}

	t.Run("known source with wrong action", func(t *testing.T) {
		if domain.IsValidAssignmentTransition(domain.AssignmentStatusOffered, domain.AssignmentStatusCompleted, "accept") {
			t.Error("IsValidAssignmentTransition(OFFERED -> COMPLETED via accept) = true, want false")
		}
	})
}

// TestAssignmentCancelCoversAllOuterBranches cancels from each status the
// outer switch accepts (including the inner per-status bookkeeping switch).
func TestAssignmentCancelCoversAllOuterBranches(t *testing.T) {
	tests := []struct {
		name   string
		status domain.AssignmentStatus
	}{
		{"cancel offered", domain.AssignmentStatusOffered},
		{"cancel accepted", domain.AssignmentStatusAccepted},
		{"cancel active", domain.AssignmentStatusActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &domain.Assignment{
				ID:         "assignment-1",
				DriverID:   "driver-1",
				DeliveryID: "delivery-1",
				Status:     tt.status,
				AcceptedAt: timePtr(time.Now()),
			}
			if err := a.Cancel(); err != nil {
				t.Fatalf("Cancel() from %s err = %v", tt.status, err)
			}
			if a.Status != domain.AssignmentStatusCancelled {
				t.Errorf("status = %s, want CANCELLED", a.Status)
			}
			if a.UpdatedAt.IsZero() {
				t.Errorf("UpdatedAt not set")
			}
			// The transition details are recorded from the status the
			// assignment had *before* it became CANCELLED.
			switch tt.status {
			case domain.AssignmentStatusOffered:
				if a.RejectedAt == nil {
					t.Error("RejectedAt = nil after cancelling an offered assignment")
				}
				if a.AcceptedAt == nil {
					t.Error("AcceptedAt = nil after cancelling an offered assignment")
				}
			case domain.AssignmentStatusAccepted:
				if a.AcceptedAt != nil {
					t.Errorf("AcceptedAt = %v after cancelling an accepted assignment, want nil", a.AcceptedAt)
				}
				if a.RejectedAt != nil {
					t.Errorf("RejectedAt = %v after cancelling an accepted assignment, want nil", a.RejectedAt)
				}
			case domain.AssignmentStatusActive:
				if a.RejectedAt != nil {
					t.Errorf("RejectedAt = %v after cancelling an active assignment, want nil", a.RejectedAt)
				}
			}
		})
	}

	t.Run("cancel from terminal status fails", func(t *testing.T) {
		a := &domain.Assignment{ID: "a", Status: domain.AssignmentStatusCompleted}
		if err := a.Cancel(); err == nil {
			t.Errorf("Cancel() from COMPLETED = nil, want error")
		}
	})
}

func timePtr(t time.Time) *time.Time { return &t }

// TestAssignmentDurationEdgeCases covers the Completed-without-CompletedAt
// path and the default path for unknown statuses.
func TestAssignmentDurationEdgeCases(t *testing.T) {
	t.Run("completed without completedAt uses UpdatedAt", func(t *testing.T) {
		updatedAt := time.Now().Add(-2 * time.Second)
		a := &domain.Assignment{
			Status:    domain.AssignmentStatusCompleted,
			UpdatedAt: updatedAt,
		}
		d := a.Duration()
		if d < 2*time.Second {
			t.Errorf("Duration() = %v, want >= 2s since UpdatedAt", d)
		}
	})

	t.Run("completed with completedAt uses completedAt", func(t *testing.T) {
		completedAt := time.Now().Add(-3 * time.Second)
		a := &domain.Assignment{
			Status:      domain.AssignmentStatusCompleted,
			CompletedAt: &completedAt,
			UpdatedAt:   time.Now(),
		}
		d := a.Duration()
		if d < 3*time.Second {
			t.Errorf("Duration() = %v, want >= 3s since CompletedAt", d)
		}
	})

	t.Run("default status returns zero", func(t *testing.T) {
		for _, status := range []domain.AssignmentStatus{
			domain.AssignmentStatusNone,
			domain.AssignmentStatus("GARBAGE"),
			domain.AssignmentStatus(""),
		} {
			a := &domain.Assignment{Status: status}
			if d := a.Duration(); d != 0 {
				t.Errorf("Duration(%s) = %v, want 0", status, d)
			}
		}
	})

	t.Run("accepted uses acceptedAt", func(t *testing.T) {
		acceptedAt := time.Now().Add(-time.Second)
		a := &domain.Assignment{
			Status:     domain.AssignmentStatusAccepted,
			AcceptedAt: &acceptedAt,
		}
		if d := a.Duration(); d < time.Second {
			t.Errorf("Duration() = %v, want >= 1s", d)
		}
	})

	t.Run("rejected expired cancelled return zero", func(t *testing.T) {
		for _, status := range []domain.AssignmentStatus{
			domain.AssignmentStatusRejected,
			domain.AssignmentStatusExpired,
			domain.AssignmentStatusCancelled,
		} {
			a := &domain.Assignment{Status: status}
			if d := a.Duration(); d != 0 {
				t.Errorf("Duration(%s) = %v, want 0", status, d)
			}
		}
	})
}

// TestDispatchPolicyRecordAttempt covers the persistence hook.
func TestDispatchPolicyRecordAttempt(t *testing.T) {
	p := domain.NewDispatchPolicy()
	p.RecordAttempt(domain.DispatchAttempt{
		ID:            "attempt-1",
		DeliveryID:    "delivery-1",
		DriverID:      "driver-1",
		AttemptNumber: 1,
		Result:        domain.DispatchResultRejected,
		Reason:        "busy",
		CreatedAt:     time.Now(),
	})
}

// TestErrorHelpers covers IsCode and Unwrap on the domain error type.
func TestErrorHelpers(t *testing.T) {
	err := &domain.Error{Code: "some_code", Message: "some message"}

	t.Run("Error string", func(t *testing.T) {
		if got := err.Error(); got != "[some_code] some message" {
			t.Errorf("Error() = %q, want %q", got, "[some_code] some message")
		}
	})

	t.Run("IsCode true", func(t *testing.T) {
		if !err.IsCode("some_code") {
			t.Errorf("IsCode(some_code) = false, want true")
		}
	})

	t.Run("IsCode false", func(t *testing.T) {
		if err.IsCode("other_code") {
			t.Errorf("IsCode(other_code) = true, want false")
		}
	})

	t.Run("Unwrap returns itself", func(t *testing.T) {
		if unwrapped := errors.Unwrap(err); unwrapped != error(err) {
			t.Errorf("Unwrap() = %v, want the error itself", unwrapped)
		}
	})

	t.Run("sentinel errors keep their codes", func(t *testing.T) {
		if !domain.ErrDriverNotFound.IsCode("driver_not_found") {
			t.Errorf("ErrDriverNotFound code mismatch: %v", domain.ErrDriverNotFound)
		}
		if !domain.ErrAssignmentExpired.IsCode("assignment_expired") {
			t.Errorf("ErrAssignmentExpired code mismatch: %v", domain.ErrAssignmentExpired)
		}
	})
}

// TestValidateTransitionAssignmentTypeErrors covers the type-assertion
// failure branches of the assignment arm in ValidateTransition.
func TestValidateTransitionAssignmentTypeErrors(t *testing.T) {
	t.Run("from has wrong type", func(t *testing.T) {
		err := domain.ValidateTransition("assignment", "not-a-status", domain.AssignmentStatusAccepted, "accept")
		if err == nil {
			t.Fatalf("ValidateTransition() = nil, want type error")
		}
		if !strings.Contains(err.Error(), "invalid assignment status type") {
			t.Errorf("err = %q, want invalid assignment status type", err.Error())
		}
	})

	t.Run("to has wrong type", func(t *testing.T) {
		err := domain.ValidateTransition("assignment", domain.AssignmentStatusOffered, 42, "accept")
		if err == nil {
			t.Fatalf("ValidateTransition() = nil, want type error")
		}
		if !strings.Contains(err.Error(), "invalid assignment status type") {
			t.Errorf("err = %q, want invalid assignment status type", err.Error())
		}
	})

	t.Run("valid assignment transition still works", func(t *testing.T) {
		if err := domain.ValidateTransition("assignment", domain.AssignmentStatusOffered, domain.AssignmentStatusAccepted, "accept"); err != nil {
			t.Errorf("ValidateTransition() = %v, want nil", err)
		}
	})

	t.Run("unsupported aggregate", func(t *testing.T) {
		err := domain.ValidateTransition("delivery", domain.AssignmentStatusOffered, domain.AssignmentStatusAccepted, "accept")
		if err == nil || !strings.Contains(err.Error(), "unsupported aggregate type") {
			t.Errorf("ValidateTransition() = %v, want unsupported aggregate type", err)
		}
	})
}
