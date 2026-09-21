package domain

import (
	"strings"
	"time"
)

type IdempotencyRecord struct {
	EventID       string
	AggregateType string
	AggregateID   string
	SourceTopic   string
	SourceOffset  int64
	FirstSeenAt   time.Time
}

func NewIdempotencyRecord(eventID, aggregateType, aggregateID, sourceTopic string, sourceOffset int64) *IdempotencyRecord {
	return &IdempotencyRecord{
		EventID:       eventID,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		SourceTopic:   sourceTopic,
		SourceOffset:  sourceOffset,
		FirstSeenAt:   time.Now().UTC(),
	}
}

func (r *IdempotencyRecord) Validate() error {
	if r == nil {
		return ErrInvalidEnvelope
	}
	if strings.TrimSpace(r.EventID) == "" {
		return ErrMissingEventID
	}
	if r.FirstSeenAt.IsZero() {
		return ErrInvalidOccurredAt
	}
	return nil
}

type IngestionCheckpoint struct {
	Topic     string
	Partition int32
	Offset    int64
	UpdatedAt time.Time
}

func NewIngestionCheckpoint(topic string, partition int32, offset int64) *IngestionCheckpoint {
	return &IngestionCheckpoint{
		Topic:     topic,
		Partition: partition,
		Offset:    offset,
		UpdatedAt: time.Now().UTC(),
	}
}

func (c *IngestionCheckpoint) Validate() error {
	if c == nil {
		return ErrInvalidCheckpoint
	}
	if strings.TrimSpace(c.Topic) == "" {
		return ErrInvalidCheckpoint
	}
	if c.Offset < 0 {
		return ErrInvalidCheckpoint
	}
	if c.UpdatedAt.IsZero() {
		return ErrInvalidCheckpoint
	}
	return nil
}

func (c *IngestionCheckpoint) Advance(offset int64, at time.Time) bool {
	if c == nil || offset <= c.Offset {
		return false
	}
	c.Offset = offset
	c.UpdatedAt = at
	return true
}
