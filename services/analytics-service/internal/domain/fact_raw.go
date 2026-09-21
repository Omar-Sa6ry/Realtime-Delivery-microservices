package domain

import (
	"strings"
	"time"
)

const RawEventRetentionTTL = 90 * 24 * time.Hour

type RawEventLanding struct {
	EventID         string
	EventType       string
	EventVersion    EventVersion
	AggregateType   string
	AggregateID     string
	Producer        string
	OccurredAt      time.Time
	IngestedAt      time.Time
	CorrelationID   string
	CausationID     string
	SourceTopic     string
	SourcePartition int32
	SourceOffset    int64
	PayloadJSON     string
}

func (r *RawEventLanding) Validate() error {
	if r == nil {
		return ErrInvalidEnvelope
	}
	if strings.TrimSpace(r.EventID) == "" {
		return ErrMissingEventID
	}
	if strings.TrimSpace(r.EventType) == "" {
		return ErrMissingEventType
	}
	if strings.TrimSpace(r.AggregateID) == "" {
		return ErrMissingAggregateID
	}
	if !r.EventVersion.IsSupported() {
		return ErrUnsupportedEventVersion
	}
	if r.OccurredAt.IsZero() {
		return ErrInvalidOccurredAt
	}
	return nil
}

func (r *RawEventLanding) ExpiresAt() time.Time {
	return r.OccurredAt.Add(RawEventRetentionTTL)
}

func (r *RawEventLanding) IsExpired(now time.Time) bool {
	return !now.Before(r.ExpiresAt())
}
