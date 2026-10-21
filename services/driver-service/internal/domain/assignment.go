package domain

import (
	"errors"
	"sync"
	"time"
)

type Assignment struct {
	mu            sync.Mutex
	ID            string           `json:"id" bson:"_id"`
	DriverID      string           `json:"driverId" bson:"driverId"`
	DeliveryID    string           `json:"deliveryId" bson:"deliveryId"`
	CustomerID    string           `json:"customerId,omitempty" bson:"customerId,omitempty"`
	Status        AssignmentStatus `json:"status" bson:"status"`
	AttemptNumber int              `json:"attemptNumber" bson:"attemptNumber"`
	OfferedAt     time.Time        `json:"offeredAt" bson:"offeredAt"`
	ExpiresAt     time.Time        `json:"expiresAt" bson:"expiresAt"`
	AcceptedAt    *time.Time       `json:"acceptedAt,omitempty" bson:"acceptedAt,omitempty"`
	RejectedAt    *time.Time       `json:"rejectedAt,omitempty" bson:"rejectedAt,omitempty"`
	CompletedAt   *time.Time       `json:"completedAt,omitempty" bson:"completedAt,omitempty"`
	CreatedAt     time.Time        `json:"createdAt" bson:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt" bson:"updatedAt"`
}

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

type AssignmentTransition struct {
	From   AssignmentStatus
	Action string
	To     AssignmentStatus
}

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

func (a *Assignment) CreateAssignment() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !IsValidAssignmentTransition(a.Status, AssignmentStatusOffered, "create") {
		return errors.New("invalid assignment state transition")
	}
	a.Status = AssignmentStatusOffered
	a.AttemptNumber++
	a.OfferedAt = time.Now()
	// Set expiration: default 30 seconds from now
	a.ExpiresAt = time.Now().Add(30 * time.Second)
	a.UpdatedAt = time.Now()
	return nil
}

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

func (s AssignmentStatus) IsValid() bool {
	return validAssignmentStatuses[s]
}

func (s AssignmentStatus) String() string {
	return string(s)
}

func (a *Assignment) Duration() time.Duration {
	switch a.Status {
	case AssignmentStatusOffered:
		return time.Since(a.OfferedAt)
	case AssignmentStatusAccepted:
		return time.Since(*a.AcceptedAt)
	case AssignmentStatusActive:
		return time.Since(a.OfferedAt)
	case AssignmentStatusCompleted:
		if a.CompletedAt != nil {
			return time.Since(*a.CompletedAt)
		}
		return time.Since(a.UpdatedAt)
	case AssignmentStatusRejected, AssignmentStatusExpired, AssignmentStatusCancelled:
		return 0
	default:
		return 0
	}
}

func (a *Assignment) ValidateOfferExpires() bool {
	return time.Now().Before(a.ExpiresAt)
}

func IsAssignmentStatusValid(s AssignmentStatus) bool {
	return s.IsValid()
}