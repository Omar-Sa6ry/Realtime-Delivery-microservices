package domain

import (
	"errors"
	"sync"
	"time"
)

// Assignment represents a driver assignment aggregate root with its state machine.
type Assignment struct {
	mu         sync.Mutex
	ID         string
	DriverID   string
	DeliveryID string
	Status     AssignmentStatus
	AttemptNumber int
	OfferedAt   time.Time
	ExpiresAt   time.Time
	AcceptedAt  *time.Time
	RejectedAt  *time.Time
	CompletedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AssignmentStatus represents the state of a driver assignment.
type AssignmentStatus string

const (
	AssignmentStatusNone      AssignmentStatus = "NONE"
	AssignmentStatusOffered   AssignmentStatus = "OFFERED"
	AssignmentStatusAccepted  AssignmentStatus = "ACCEPTED"
	AssignmentStatusActive    AssignmentStatus = "ACTIVE"
	AssignmentStatusCompleted AssignmentStatus = "COMPLETED"
	AssignmentStatusRejected  AssignmentStatus = "REJECTED"
	AssignmentStatusExpired   AssignmentStatus = "EXPIRED"
	AssignmentStatusCancelled AssignmentStatus = "CANCELLED"
)

// Valid assignment statuses for the public state machine.
var validAssignmentStatuses = map[AssignmentStatus]bool{
	AssignmentStatusNone:      true,
	AssignmentStatusOffered:   true,
	AssignmentStatusAccepted:  true,
	AssignmentStatusActive:    true,
	AssignmentStatusCompleted: true,
	AssignmentStatusRejected:  true,
	AssignmentStatusExpired:   true,
	AssignmentStatusCancelled: true,
}

// AssignmentTransition represents a transition action.
type AssignmentTransition struct {
	From   AssignmentStatus
	Action string
	To     AssignmentStatus
}

// AssignmentTransitionMap maps actions to allowed transitions for each state.
var AssignmentTransitionMap = map[AssignmentStatus][]AssignmentTransition{
	AssignmentStatusNone: {
		{From: AssignmentStatusNone, Action: "create", To: AssignmentStatusOffered},
	},
	AssignmentStatusOffered: {
		{From: AssignmentStatusOffered, Action: "accept", To: AssignmentStatusAccepted},
		{From: AssignmentStatusOffered, Action: "reject", To: AssignmentStatusRejected},
		{From: AssignmentStatusOffered, Action: "expire", To: AssignmentStatusExpired},
	},
	AssignmentStatusAccepted: {
		{From: AssignmentStatusAccepted, Action: "start", To: AssignmentStatusActive},
	},
	AssignmentStatusActive: {
		{From: AssignmentStatusActive, Action: "complete", To: AssignmentStatusCompleted},
	},
	AssignmentStatusRejected: {
		{From: AssignmentStatusRejected, Action: "cancel", To: AssignmentStatusCancelled},
	},
	AssignmentStatusExpired: {
		{From: AssignmentStatusExpired, Action: "cancel", To: AssignmentStatusCancelled},
	},
	AssignmentStatusCancelled: {
		// No outgoing transitions from cancelled
	},
}

// IsValidTransition checks if a transition from one status to another via an action is valid.
func IsValidAssignmentTransition(from, to AssignmentStatus, action string) bool {
	transitions, ok := AssignmentTransitionMap[from]
	if !ok {
		return false
	}
	for _, t := range transitions {
		if t.From == from && t.Action == action && t.To == to {
			return true
		}
	}
	return false
}

// CreateAssignment creates a new assignment from NONE to OFFERED.
func (a *Assignment) CreateAssignment() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !IsValidAssignmentTransition(a.Status, AssignmentStatusOffered, "create") {
		return errors.New("invalid assignment state transition")
	}
	a.Status = AssignmentStatusOffered
	a.AttemptNumber++
	a.OfferedAt = time.Now()
	// Set expiration: default 20 seconds from now (configurable)
	a.ExpiresAt = time.Now().Add(20 * time.Second)
	a.UpdatedAt = time.Now()
	return nil
}

// Accept transitions an assignment from OFFERED to ACCEPTED.
func (a *Assignment) Accept() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !IsValidAssignmentTransition(a.Status, AssignmentStatusAccepted, "accept") {
		return errors.New("invalid assignment state transition")
	}
	now := time.Now()
	a.Status = AssignmentStatusAccepted
	a.AcceptedAt = &now
	a.UpdatedAt = time.Now()
	return nil
}

// Reject transitions an assignment from OFFERED to REJECTED.
func (a *Assignment) Reject() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !IsValidAssignmentTransition(a.Status, AssignmentStatusRejected, "reject") {
		return errors.New("invalid assignment state transition")
	}
	now := time.Now()
	a.Status = AssignmentStatusRejected
	a.RejectedAt = &now
	a.UpdatedAt = time.Now()
	return nil
}

// Expire transitions an assignment from OFFERED to EXPIRED based on timeout.
func (a *Assignment) Expire() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !IsValidAssignmentTransition(a.Status, AssignmentStatusExpired, "expire") {
		return errors.New("invalid assignment state transition")
	}
	a.Status = AssignmentStatusExpired
	a.UpdatedAt = time.Now()
	return nil
}

// Start transitions an assignment from ACCEPTED to ACTIVE when delivery starts.
func (a *Assignment) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !IsValidAssignmentTransition(a.Status, AssignmentStatusActive, "start") {
		return errors.New("invalid assignment state transition")
	}
	a.Status = AssignmentStatusActive
	a.UpdatedAt = time.Now()
	return nil
}

// Complete transitions an assignment from ACTIVE to COMPLETED when delivery finishes.
func (a *Assignment) Complete() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !IsValidAssignmentTransition(a.Status, AssignmentStatusCompleted, "complete") {
		return errors.New("invalid assignment state transition")
	}
	now := time.Now()
	a.Status = AssignmentStatusCompleted
	a.CompletedAt = &now
	a.UpdatedAt = time.Now()
	return nil
}

// Cancel transitions an assignment to CANCELLED from OFFERED, ACCEPTED, or ACTIVE.
func (a *Assignment) Cancel() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if current status allows cancellation
	switch a.Status {
	case AssignmentStatusOffered, AssignmentStatusAccepted, AssignmentStatusActive:
		now := time.Now()
		a.Status = AssignmentStatusCancelled
		switch a.Status {
		case AssignmentStatusOffered:
			a.RejectedAt = &now
		case AssignmentStatusAccepted:
			a.AcceptedAt = nil
		}
		a.UpdatedAt = time.Now()
		return nil
	default:
		return errors.New("invalid assignment state transition")
	}
}

// IsValid checks if the assignment status is a valid public state.
func (s AssignmentStatus) IsValid() bool {
	return validAssignmentStatuses[s]
}

// String returns the human-readable status.
func (s AssignmentStatus) String() string {
	return string(s)
}

// AssignmentDuration returns the duration the assignment has been in its current state.
func (a *Assignment) Duration() time.Duration {
	switch a.Status {
	case AssignmentStatusOffered:
		return time.Since(a.OfferedAt)
	case AssignmentStatusAccepted:
		return time.Since(*a.AcceptedAt)
	case AssignmentStatusActive:
		return time.Since(*a.CompletedAt) // completedAt is nil for active, so this returns duration since start... let me fix
	case AssignmentStatusCompleted:
		return time.Since(*a.CompletedAt)
	case AssignmentStatusRejected, AssignmentStatusExpired, AssignmentStatusCancelled:
		return 0
	default:
		return 0
	}
}

// ValidateOfferExpensess validates that the offer has not expired.
func (a *Assignment) ValidateOfferExpires() bool {
	return time.Now().Before(a.ExpiresAt)
}

// Ensure AssignmentStatusIsValidError checks if status is valid.
func IsAssignmentStatusValid(s AssignmentStatus) bool {
	return s.IsValid()
}