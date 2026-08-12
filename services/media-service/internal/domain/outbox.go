package domain

import "time"

// OutboxStatus tracks whether a domain event has been published to Kafka.
type OutboxStatus string

const (
	OutboxStatusPending   OutboxStatus = "PENDING"
	OutboxStatusPublished OutboxStatus = "PUBLISHED"
	OutboxStatusFailed    OutboxStatus = "FAILED"
)

// OutboxEvent stores a domain event that must be durably published to Kafka.
// It is written in the same DynamoDB transaction as the state change, guaranteeing
// at-least-once delivery even if Kafka is temporarily unavailable.
// Published events are automatically expired by DynamoDB TTL after 30 days.
type OutboxEvent struct {
	EventID     string       `json:"eventId"`
	AggregateID string       `json:"aggregateId"` // typically mediaId
	EventType   string       `json:"eventType"`
	Payload     []byte       `json:"payload"` // JSON-encoded event body
	Status      OutboxStatus `json:"status"`
	Attempts    int          `json:"attempts"`
	TraceID     string       `json:"traceId,omitempty"`
	PublishedAt *time.Time   `json:"publishedAt,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	// TTL is a Unix timestamp used by DynamoDB TTL to auto-expire old records.
	TTL int64 `json:"ttl"`
}
