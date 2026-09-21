package domain

import (
	"fmt"
	"strings"
	"time"
)

type EventVersion int

const (
	MaxSupportedEventVersion EventVersion = 1
	FutureTimestampTolerance              = 5 * time.Minute
)

// IsSupported reports whether v can be processed by this service version.
func (v EventVersion) IsSupported() bool {
	return v >= 1 && v <= MaxSupportedEventVersion
}

type EventEnvelope struct {
	EventID       string
	EventType     string
	EventVersion  EventVersion
	OccurredAt    time.Time
	Producer      string
	AggregateType string
	AggregateID   string
	CorrelationID string
	CausationID   string

	// Ingestion lineage.
	SourceTopic     string
	SourcePartition int32
	SourceOffset    int64
	IngestedAt      time.Time
}

func (e *EventEnvelope) Validate() error {
	if e == nil {
		return ErrInvalidEnvelope
	}
	if strings.TrimSpace(e.EventID) == "" {
		return ErrMissingEventID
	}
	if strings.TrimSpace(e.EventType) == "" {
		return ErrMissingEventType
	}
	if strings.TrimSpace(e.AggregateID) == "" {
		return ErrMissingAggregateID
	}
	if !e.EventVersion.IsSupported() {
		return fmt.Errorf("%w: %d (max supported %d)", ErrUnsupportedEventVersion, e.EventVersion, MaxSupportedEventVersion)
	}
	if e.OccurredAt.IsZero() {
		return ErrInvalidOccurredAt
	}
	if e.OccurredAt.After(time.Now().UTC().Add(FutureTimestampTolerance)) {
		return fmt.Errorf("%w: occurredAt %s", ErrEventFromFuture, e.OccurredAt.Format(time.RFC3339))
	}
	return nil
}

var KnownAggregateTypes = map[string]bool{
	"delivery":     true,
	"driver":       true,
	"assignment":   true,
	"payment":      true,
	"notification": true,
	"user":         true,
}

func IsKnownAggregateType(aggregateType string) bool {
	return KnownAggregateTypes[strings.ToLower(strings.TrimSpace(aggregateType))]
}
