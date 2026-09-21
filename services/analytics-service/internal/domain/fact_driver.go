package domain

import (
	"fmt"
	"strings"
	"time"
)

type AssignmentResult string

const (
	AssignmentOffered  AssignmentResult = "OFFERED"
	AssignmentAccepted AssignmentResult = "ACCEPTED"
	AssignmentRejected AssignmentResult = "REJECTED"
	AssignmentExpired  AssignmentResult = "EXPIRED"
	AssignmentReleased AssignmentResult = "RELEASED"
)

func (r AssignmentResult) IsValid() bool {
	switch r {
	case AssignmentOffered, AssignmentAccepted, AssignmentRejected, AssignmentExpired, AssignmentReleased:
		return true
	default:
		return false
	}
}

func (r AssignmentResult) IsTerminal() bool {
	switch r {
	case AssignmentAccepted, AssignmentRejected, AssignmentExpired, AssignmentReleased:
		return true
	default:
		return false
	}
}

type FactDriverAssignment struct {
	AssignmentID   string
	DeliveryID     string
	DriverID       string
	OfferedAt      time.Time
	AcceptedAt     *time.Time
	RejectedAt     *time.Time
	ExpiredAt      *time.Time
	ReleasedAt     *time.Time
	DistanceMeters *uint32
	ResponseTimeMs *uint32
	Result         AssignmentResult
	IngestedAt     time.Time
}

func (f *FactDriverAssignment) ComputeResponseTime() {
	if f == nil || f.OfferedAt.IsZero() {
		return
	}
	var respondedAt *time.Time
	for _, t := range []*time.Time{f.AcceptedAt, f.RejectedAt, f.ExpiredAt, f.ReleasedAt} {
		if t != nil && !t.Before(f.OfferedAt) && (respondedAt == nil || t.Before(*respondedAt)) {
			respondedAt = t
		}
	}
	if respondedAt == nil {
		f.ResponseTimeMs = nil
		return
	}
	ms := uint32(respondedAt.Sub(f.OfferedAt).Milliseconds())
	f.ResponseTimeMs = &ms
}

func (f *FactDriverAssignment) Validate() error {
	if f == nil {
		return ErrInvalidEnvelope
	}
	if strings.TrimSpace(f.AssignmentID) == "" {
		return ErrMissingAssignmentID
	}
	if strings.TrimSpace(f.DriverID) == "" {
		return ErrMissingDriverID
	}
	if !f.Result.IsValid() {
		return fmt.Errorf("%w: assignment_result %s", ErrUnknownEventType, f.Result)
	}
	if f.OfferedAt.IsZero() {
		return ErrInvalidOccurredAt
	}
	for name, t := range map[string]*time.Time{
		"accepted_at": f.AcceptedAt,
		"rejected_at": f.RejectedAt,
		"expired_at":  f.ExpiredAt,
		"released_at": f.ReleasedAt,
	} {
		if t != nil && t.Before(f.OfferedAt) {
			return fmt.Errorf("%w: %s before offered_at", ErrOutOfOrderTimestamps, name)
		}
	}
	return nil
}
