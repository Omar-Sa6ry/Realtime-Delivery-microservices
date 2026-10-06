package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	kafkago "github.com/segmentio/kafka-go"
)

type EventEnvelope = events.EventEnvelope

func MarshalEnvelope(eventID, eventType, traceID string, payload interface{}) ([]byte, error) {
	return events.MarshalEnvelope(eventID, eventType, traceID, payload)
}

func UnmarshalEnvelope(data []byte) (*EventEnvelope, error) {
	return events.UnmarshalEnvelope(data)
}

type MessageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafkago.Message) error
	Close() error
}

type MessageReader interface {
	FetchMessage(ctx context.Context) (kafkago.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafkago.Message) error
	Config() kafkago.ReaderConfig
	Close() error
}

type TopicConn interface {
	CreateTopics(topics ...kafkago.TopicConfig) error
	Close() error
}

var dialKafka = func(network, address string) (TopicConn, error) {
	return kafkago.Dial(network, address)
}

type Producer struct {
	writer MessageWriter
}

func NewProducer(brokers []string) *Producer {
	writer := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireAll,
		MaxAttempts:  3,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		Compression:  kafkago.Snappy,
		Logger: kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			slog.Debug(fmt.Sprintf(msg, args...), "component", "kafka-producer")
		}),
		ErrorLogger: kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			slog.Error(fmt.Sprintf(msg, args...), "component", "kafka-producer")
		}),
	}
	return &Producer{writer: writer}
}

func (p *Producer) Publish(ctx context.Context, topic, key string, payload []byte, traceID string) error {
	msg := kafkago.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
		Headers: []kafkago.Header{
			{Key: "x-trace-id", Value: []byte(traceID)},
			{Key: "x-timestamp", Value: []byte(fmt.Sprintf("%d", time.Now().UnixMilli()))},
		},
		Time: time.Now(),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("kafka publish to topic %q: %w", topic, err)
	}
	return nil
}

func (p *Producer) PublishEnvelope(ctx context.Context, topic, key, eventType, traceID string, payload interface{}) error {
	data, err := MarshalEnvelope("", eventType, traceID, payload)
	if err != nil {
		return fmt.Errorf("kafka envelope marshal: %w", err)
	}
	return p.Publish(ctx, topic, key, data, traceID)
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

func EnsureTopics(brokers []string, topics []string, numPartitions, replicationFactor int) error {
	if len(brokers) == 0 {
		return fmt.Errorf("dial kafka for topic creation: no brokers provided")
	}
	conn, err := dialKafka("tcp", brokers[0])
	if err != nil {
		return fmt.Errorf("dial kafka for topic creation: %w", err)
	}
	defer conn.Close()

	topicConfigs := make([]kafkago.TopicConfig, 0, len(topics))
	for _, t := range topics {
		topicConfigs = append(topicConfigs, kafkago.TopicConfig{
			Topic:             t,
			NumPartitions:     numPartitions,
			ReplicationFactor: replicationFactor,
		})
	}

	err = conn.CreateTopics(topicConfigs...)
	if err != nil {
		// TopicAlreadyExists is fine
		if strings.Contains(err.Error(), "TopicAlreadyExists") {
			return nil
		}
		return fmt.Errorf("create topics: %w", err)
	}
	return nil
}

type MessageHandler func(ctx context.Context, msg kafkago.Message) error

var ErrPermanent = errors.New("permanent processing failure")

type ConsumerConfig struct {
	Brokers    []string
	Topic      string
	GroupID    string
	MaxRetries int
	DLQ        *Producer // may be nil — failed messages are just logged
}

type Consumer struct {
	reader      MessageReader
	maxRetries  int
	retryDelay  time.Duration
	dlqProducer *Producer
}

func NewConsumer(cfg ConsumerConfig) *Consumer {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 0,
		MaxWait:        500 * time.Millisecond,
		StartOffset:    kafkago.FirstOffset,
		Logger: kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			slog.Debug(fmt.Sprintf(msg, args...), "component", "kafka-consumer", "topic", cfg.Topic)
		}),
		ErrorLogger: kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			formatted := fmt.Sprintf(msg, args...)
			if strings.Contains(formatted, "Rebalance In Progress") || 
			   strings.Contains(formatted, "i/o timeout") || 
			   strings.Contains(formatted, "Not Coordinator For Group") ||
			   strings.Contains(formatted, "Group Coordinator Not Available") ||
			   strings.Contains(formatted, "Not Leader For Partition") {
				slog.Debug(formatted, "component", "kafka-consumer", "topic", cfg.Topic)
			} else {
				slog.Error(formatted, "component", "kafka-consumer", "topic", cfg.Topic)
			}
		}),
	})

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	return &Consumer{
		reader:      reader,
		maxRetries:  maxRetries,
		retryDelay:  2 * time.Second,
		dlqProducer: cfg.DLQ,
	}
}

func (c *Consumer) Run(ctx context.Context, handler MessageHandler) error {
	slog.Info("Kafka consumer started", "topic", c.reader.Config().Topic, "group", c.reader.Config().GroupID)
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				slog.Info("Kafka consumer stopped", "topic", c.reader.Config().Topic)
				return nil
			}
			if strings.Contains(err.Error(), "Rebalance In Progress") || 
			   strings.Contains(err.Error(), "Not Coordinator For Group") ||
			   strings.Contains(err.Error(), "Group Coordinator Not Available") ||
			   strings.Contains(err.Error(), "Not Leader For Partition") {
				slog.Debug("Kafka consumer transient coordinator/rebalance/leader state, waiting...", "topic", c.reader.Config().Topic)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(500 * time.Millisecond):
				}
				continue
			}
			slog.Error("Failed to fetch Kafka message", "error", err, "topic", c.reader.Config().Topic)
			continue
		}

		if processErr := c.processWithRetry(ctx, msg, handler); processErr != nil {
			c.routeToDLQ(ctx, msg, processErr)
		}

		if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
			slog.Error("Failed to commit Kafka offset", "error", commitErr, "topic", c.reader.Config().Topic)
		}
	}
}

func (c *Consumer) processWithRetry(ctx context.Context, msg kafkago.Message, handler MessageHandler) error {
	var lastErr error
	delay := c.retryDelay

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("Retrying Kafka message processing",
				"attempt", attempt,
				"topic", c.reader.Config().Topic,
				"offset", msg.Offset,
				"delay_ms", delay.Milliseconds(),
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
		}

		lastErr = handler(ctx, msg)
		if lastErr == nil {
			return nil
		}
		if errors.Is(lastErr, ErrPermanent) {
			return lastErr
		}
	}

	return fmt.Errorf("exhausted %d retries: %w", c.maxRetries, lastErr)
}

func (c *Consumer) routeToDLQ(ctx context.Context, msg kafkago.Message, reason error) {
	topic := c.reader.Config().Topic
	slog.Error("Routing Kafka message to DLQ",
		"topic", topic,
		"offset", msg.Offset,
		"reason", reason.Error(),
	)

	if c.dlqProducer == nil {
		return
	}

	headers := append(msg.Headers, kafkago.Header{Key: "x-dlq-reason", Value: []byte(reason.Error())})
	dlqMsg := kafkago.Message{
		Topic:   topic + ".dlq",
		Key:     msg.Key,
		Value:   msg.Value,
		Headers: headers,
	}
	if err := c.dlqProducer.writer.WriteMessages(ctx, dlqMsg); err != nil {
		slog.Error("Failed to write to DLQ", "error", err, "dlq_topic", topic+".dlq")
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
