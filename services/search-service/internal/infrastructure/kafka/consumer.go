package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/config"
	kafkago "github.com/segmentio/kafka-go"
)

type ConsumerManager struct {
	cfg             *config.Config
	indexingService *indexing.Service
	consumers       []*kafka.Consumer
	dlqProducer     *kafka.Producer
	handlerRegistry *handlerRegistry
}

func NewConsumerManager(cfg *config.Config, indexingService *indexing.Service) *ConsumerManager {
	dlqProducer := kafka.NewProducer(cfg.KafkaBrokers)
	return &ConsumerManager{
		cfg:             cfg,
		indexingService: indexingService,
		dlqProducer:     dlqProducer,
		handlerRegistry: BuildRegistry(indexingService),
	}
}

func (cm *ConsumerManager) Start(ctx context.Context) error {
	topics := []string{
		// Delivery
		"delivery.created",
		"delivery.driver.assigned",
		"delivery.driver.accepted",
		"delivery.picked_up",
		"delivery.in_transit",
		"delivery.completed",
		"delivery.cancelled",
		"delivery.deleted",
		// Driver
		"driver.created",
		"driver.updated",
		"driver.deleted",
		// Media
		"media.upload.created",
		"media.upload.completed",
		"media.ready",
		"media.deleted",
		// User
		"user.created",
		"user.updated",
		"user.deleted",
	}

	var wg sync.WaitGroup
	for _, topic := range topics {
		consumer := kafka.NewConsumer(kafka.ConsumerConfig{
			Brokers:    cm.cfg.KafkaBrokers,
			Topic:      topic,
			GroupID:    fmt.Sprintf("%s-%s", cm.cfg.KafkaGroupID, topic),
			MaxRetries: 3,
			DLQ:        cm.dlqProducer,
		})
		cm.consumers = append(cm.consumers, consumer)

		wg.Add(1)
		go func(t string, c *kafka.Consumer) {
			defer wg.Done()
			err := c.Run(ctx, func(handlerCtx context.Context, msg kafkago.Message) error {
				return cm.HandleEvent(handlerCtx, cm.handlerRegistry, msg)
			})
			if err != nil {
				slog.Error("Kafka consumer stopped with error", "topic", t, "error", err)
			}
		}(topic, consumer)
	}

	return nil
}

func (cm *ConsumerManager) Close() error {
	if cm.dlqProducer != nil {
		_ = cm.dlqProducer.Close()
	}
	for _, c := range cm.consumers {
		_ = c.Close()
	}
	return nil
}