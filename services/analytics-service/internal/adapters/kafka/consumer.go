package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/ingestion"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
)

const (
	baseRetryDelay = 100 * time.Millisecond
	maxRetryDelay  = 1600 * time.Millisecond
)

var ErrPermanent = errors.New("permanent processing failure")

type Handler func(ctx context.Context, topic string, msg kafkago.Message) error

type Consumer struct {
	readers    []*kafkago.Reader
	topics     []string
	groupID    string
	maxRetries int
	dlq        *DLQPublisher
}

type ConsumerConfig struct {
	Brokers    []string
	Topics     []string
	GroupID    string
	MaxRetries int
	DLQ        *DLQPublisher
}

func NewConsumer(cfg ConsumerConfig) *Consumer {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 5
	}
	c := &Consumer{
		topics:     append([]string(nil), cfg.Topics...),
		groupID:    cfg.GroupID,
		maxRetries: maxRetries,
		dlq:        cfg.DLQ,
	}
	for _, topic := range cfg.Topics {
		c.readers = append(c.readers, kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:        cfg.Brokers,
			Topic:          topic,
			GroupID:        cfg.GroupID,
			MinBytes:       1,
			MaxBytes:       10e6,
			CommitInterval: 0,
			MaxWait:        500 * time.Millisecond,
			StartOffset:    kafkago.FirstOffset,
		}))
	}
	return c
}

func (c *Consumer) Run(ctx context.Context, handler Handler) error {
	errCh := make(chan error, len(c.readers))
	for i, r := range c.readers {
		go func(topic string, reader *kafkago.Reader) {
			errCh <- c.consumeLoop(ctx, topic, reader, handler)
		}(c.topics[i], r)
	}
	for range c.readers {
		if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	return nil
}

func (c *Consumer) consumeLoop(ctx context.Context, topic string, reader *kafkago.Reader, handler Handler) error {
	slog.Info("analytics kafka consumer started", "topic", topic, "group", c.groupID)
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			slog.Error("fetch kafka message failed", "topic", topic, "error", err)
			continue
		}
		if procErr := c.processWithRetry(ctx, topic, msg, handler); procErr != nil {
			c.routeToDLQ(ctx, topic, msg, procErr)
		}
		if commitErr := reader.CommitMessages(ctx, msg); commitErr != nil {
			slog.Error("commit kafka offset failed", "topic", topic, "error", commitErr)
		}
	}
}

func (c *Consumer) processWithRetry(ctx context.Context, topic string, msg kafkago.Message, handler Handler) error {
	var lastErr error
	delay := baseRetryDelay
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			jitter := time.Duration(rand.Int63n(int64(delay / 2)))
			wait := delay + jitter
			slog.Warn("retrying kafka message",
				"topic", topic, "offset", msg.Offset, "attempt", attempt, "wait_ms", wait.Milliseconds())
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			delay *= 2
			if delay > maxRetryDelay {
				delay = maxRetryDelay
			}
		}
		lastErr = handler(ctx, topic, msg)
		if lastErr == nil {
			return nil
		}
		if errors.Is(lastErr, ErrPermanent) || errors.Is(lastErr, domain.ErrUnsupportedEventVersion) || ingestion.IsPermanent(lastErr) {
			return lastErr
		}
	}
	return fmt.Errorf("exhausted %d retries: %w", c.maxRetries, lastErr)
}

func (c *Consumer) routeToDLQ(ctx context.Context, topic string, msg kafkago.Message, reason error) {
	slog.Error("routing message to DLQ", "topic", topic, "offset", msg.Offset, "reason", reason.Error())
	if c.dlq == nil {
		return
	}
	attempts := c.maxRetries + 1
	if errors.Is(reason, ErrPermanent) || ingestion.IsPermanent(reason) {
		attempts = 1
	}
	dlqMsg := DLQMessage{
		OriginalTopic: topic,
		Partition:     int32(msg.Partition),
		Offset:        msg.Offset,
		AttemptCount:  attempts,
		Error:         reason.Error(),
		Payload:       msg.Value,
	}
	for _, h := range msg.Headers {
		if h.Key == "x-trace-id" || h.Key == "x-correlation-id" {
			dlqMsg.CorrelationID = string(h.Value)
		}
	}
	if err := c.dlq.Publish(ctx, dlqMsg); err != nil {
		slog.Error("dlq publish failed", "topic", topic, "error", err)
	}
}

func (c *Consumer) Close() error {
	var firstErr error
	for _, r := range c.readers {
		if err := r.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
