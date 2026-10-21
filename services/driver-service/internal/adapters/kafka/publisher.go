package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaPublisher struct {
	writer *kafka.Writer
}

func NewKafkaPublisher(brokers []string, defaultTopic string) *KafkaPublisher {
	return &KafkaPublisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
			WriteTimeout: 10 * time.Second,
		},
	}
}

func (p *KafkaPublisher) PublishMessage(ctx context.Context, topic, key string, value []byte) error {
	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
		Headers: []kafka.Header{
			{Key: "x-source", Value: []byte("driver-service")},
			{Key: "x-timestamp", Value: []byte(fmt.Sprintf("%d", time.Now().UnixMilli()))},
		},
		Time: time.Now(),
	}
	return p.writer.WriteMessages(ctx, msg)
}

func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}