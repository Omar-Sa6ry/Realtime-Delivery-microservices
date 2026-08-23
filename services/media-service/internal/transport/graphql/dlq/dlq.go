package dlq

import (
	"context"
	"fmt"
	"log/slog"

	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
)

// DLQManager handles Dead Letter Queue operations
type DLQManager struct {
	producer ports.EventPublisher
	logger   *slog.Logger
}

// NewDLQManager creates a new DLQ manager
func NewDLQManager(producer ports.EventPublisher, logger *slog.Logger) *DLQManager {
	return &DLQManager{
		producer: producer,
		logger:   logger.With("component", "dlq-manager"),
	}
}

// DLQMessage aliases the shared port model so all adapters use one wire contract.
type DLQMessage = ports.DLQMessage

// ListDLQTopics returns all DLQ topics for media service
func (m *DLQManager) ListDLQTopics() []string {
	return []string{
		kafkaadapter.TopicUploadCompleted + ".dlq",
		kafkaadapter.TopicScanCompleted + ".dlq",
		kafkaadapter.TopicScanStarted + ".dlq",
		kafkaadapter.TopicScanFailed + ".dlq",
		kafkaadapter.TopicProcessingStarted + ".dlq",
		kafkaadapter.TopicProcessingCompleted + ".dlq",
		kafkaadapter.TopicProcessingFailed + ".dlq",
		kafkaadapter.TopicMediaReady + ".dlq",
		kafkaadapter.TopicDeleteRequested + ".dlq",
		kafkaadapter.TopicMediaDeleted + ".dlq",
		kafkaadapter.TopicDeleteFailed + ".dlq",
	}
}

// ReplayDLQMessage replays a single message from DLQ back to its original topic
func (m *DLQManager) ReplayDLQMessage(ctx context.Context, topic string, msg DLQMessage) error {
	// Remove .dlq suffix to get original topic
	originalTopic := topic
	if len(topic) > 4 && topic[len(topic)-4:] == ".dlq" {
		originalTopic = topic[:len(topic)-4]
	}

	// Extract trace ID from headers
	traceID := ""
	if msg.Headers != nil {
		traceID = msg.Headers["x-trace-id"]
	}
	if traceID == "" {
		traceID = uuid.NewString()
	}

	// Publish back to original topic
	err := m.producer.Publish(ctx, originalTopic, msg.Key, msg.Value, traceID)
	if err != nil {
		m.logger.Error("Failed to replay DLQ message", "topic", originalTopic, "error", err)
		return fmt.Errorf("replay to %s: %w", originalTopic, err)
	}

	m.logger.Info("Replayed DLQ message", "originalTopic", originalTopic, "dlqTopic", topic, "key", msg.Key)
	return nil
}

// ReplayAllDLQMessages replays all messages from a DLQ topic
func (m *DLQManager) ReplayAllDLQMessages(ctx context.Context, brokers []string, topic string, maxMessages int) (int, error) {
	dlqTopic := topic
	if len(topic) > 4 && topic[len(topic)-4:] != ".dlq" {
		dlqTopic = topic + ".dlq"
	}

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     brokers,
		Topic:       dlqTopic,
		GroupID:     "dlq-replay-" + uuid.NewString(),
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafkago.FirstOffset,
	})
	defer reader.Close()

	replayed := 0
	for i := 0; i < maxMessages; i++ {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if err == context.Canceled {
				break
			}
			m.logger.Error("Failed to read DLQ message", "dlqTopic", dlqTopic, "error", err)
			break
		}

		dlqMsg := DLQMessage{
			ID:        uuid.NewString(),
			Topic:     dlqTopic,
			Partition: msg.Partition,
			Offset:    msg.Offset,
			Key:       string(msg.Key),
			Value:     msg.Value,
			Headers:   make(map[string]string),
		}

		for _, h := range msg.Headers {
			dlqMsg.Headers[h.Key] = string(h.Value)
		}

		if err := m.ReplayDLQMessage(ctx, dlqTopic, dlqMsg); err != nil {
			m.logger.Error("Failed to replay message", "offset", msg.Offset, "error", err)
			continue
		}

		// Commit offset after successful replay
		if err := reader.CommitMessages(ctx, msg); err != nil {
			m.logger.Warn("Failed to commit DLQ offset", "offset", msg.Offset, "error", err)
		}

		replayed++
	}

	m.logger.Info("DLQ replay completed", "dlqTopic", dlqTopic, "replayed", replayed)
	return replayed, nil
}

// GetDLQStats returns statistics about DLQ messages
func (m *DLQManager) GetDLQStats(ctx context.Context, brokers []string, topics []string) (map[string]int, error) {
	stats := make(map[string]int)

	for _, topic := range topics {
		dlqTopic := topic
		if len(topic) > 4 && topic[len(topic)-4:] != ".dlq" {
			dlqTopic = topic + ".dlq"
		}

		reader := kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:  brokers,
			Topic:    dlqTopic,
			GroupID:  "dlq-stats-" + uuid.NewString(),
			MinBytes: 1,
			MaxBytes: 10e6,
		})

		// Get last offset to estimate message count
		lastOffset, err := reader.ReadMessage(ctx)
		if err == nil {
			stats[dlqTopic] = int(lastOffset.Offset) + 1
			reader.CommitMessages(ctx, lastOffset)
		} else {
			stats[dlqTopic] = 0
		}
		reader.Close()
	}

	return stats, nil
}

// Compile-time check
var _ ports.DLQManager = (*DLQManager)(nil)
