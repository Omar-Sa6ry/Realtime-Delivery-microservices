package ports

import (
	"context"
	"encoding/json"
	"time"
)

type EventPublisher interface {
	Publish(ctx context.Context, topic string, key string, payload []byte, traceID string) error
	Close() error
}

type RealtimePublisher interface {
	PublishProgress(ctx context.Context, subject string, payload interface{}) error
}

type DLQManager interface {
	ListDLQTopics() []string
	ReplayDLQMessage(ctx context.Context, topic string, msg DLQMessage) error
	ReplayAllDLQMessages(ctx context.Context, brokers []string, topic string, maxMessages int) (int, error)
	GetDLQStats(ctx context.Context, brokers []string, topics []string) (map[string]int, error)
}

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

type VirusScanner interface {
	ScanObject(ctx context.Context, objectKey string) (infected bool, threat string, err error)
}
