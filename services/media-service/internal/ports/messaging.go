package ports

import (
	"context"
	"encoding/json"
	"time"
)

type EventPublisher interface {
	// Publish sends a single event to the appropriate Kafka topic.
	// The topic is derived from the event type.
	Publish(ctx context.Context, topic string, key string, payload []byte, traceID string) error

	// Close gracefully shuts down the publisher, flushing any pending messages.
	Close() error
}

type RealtimePublisher interface {
	PublishProgress(ctx context.Context, subject string, payload interface{}) error
}

// DLQManager abstracts Dead Letter Queue operations
type DLQManager interface {
	// ListDLQTopics returns all DLQ topics for media service
	ListDLQTopics() []string

	// ReplayDLQMessage replays a single message from DLQ back to its original topic
	ReplayDLQMessage(ctx context.Context, topic string, msg DLQMessage) error

	// ReplayAllDLQMessages replays all messages from a DLQ topic
	ReplayAllDLQMessages(ctx context.Context, brokers []string, topic string, maxMessages int) (int, error)

	// GetDLQStats returns statistics about DLQ messages
	GetDLQStats(ctx context.Context, brokers []string, topics []string) (map[string]int, error)
}

// DLQMessage represents a message in the Dead Letter Queue
type DLQMessage struct {
	ID                string                 `json:"id"`
	Topic             string                 `json:"topic"`
	Partition         int                    `json:"partition"`
	Offset            int64                  `json:"offset"`
	Key               string                 `json:"key"`
	Value             json.RawMessage        `json:"value"`
	Headers           map[string]string      `json:"headers"`
	Error             string                 `json:"error"`
	RetryCount        int                    `json:"retryCount"`
	CreatedAt         time.Time              `json:"createdAt"`
	OriginalTimestamp time.Time              `json:"originalTimestamp"`
}

// VirusScanner abstracts malware scanning for uploaded objects.
// The default implementation delegates to ClamAV via TCP.
type VirusScanner interface {
	ScanObject(ctx context.Context, objectKey string) (infected bool, threat string, err error)
}
