package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	pkgKafka "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
	kafkago "github.com/segmentio/kafka-go"
)

type ConsumerManager struct {
	cfg             *config.Config
	indexingService *indexing.Service
	consumers       []*pkgKafka.Consumer
	dlqProducer     *pkgKafka.Producer
}

func NewConsumerManager(cfg *config.Config, indexingService *indexing.Service) *ConsumerManager {
	dlqProducer := pkgKafka.NewProducer(cfg.KafkaBrokers)
	return &ConsumerManager{
		cfg:             cfg,
		indexingService: indexingService,
		dlqProducer:     dlqProducer,
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
		consumer := pkgKafka.NewConsumer(pkgKafka.ConsumerConfig{
			Brokers:    cm.cfg.KafkaBrokers,
			Topic:      topic,
			GroupID:    fmt.Sprintf("%s-%s", cm.cfg.KafkaGroupID, topic),
			MaxRetries: 3,
			DLQ:        cm.dlqProducer,
		})
		cm.consumers = append(cm.consumers, consumer)

		wg.Add(1)
		go func(t string, c *pkgKafka.Consumer) {
			defer wg.Done()
			err := c.Run(ctx, func(handlerCtx context.Context, msg kafkago.Message) error {
				return cm.handleMessage(handlerCtx, t, msg)
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

func (cm *ConsumerManager) handleMessage(ctx context.Context, topic string, msg kafkago.Message) error {
	env, err := events.UnmarshalEnvelope(msg.Value)
	if err != nil {
		return fmt.Errorf("%w: invalid envelope json: %v", pkgKafka.ErrPermanent, err)
	}

	slog.Info("Consuming event for search projection", "topic", topic, "eventType", env.EventType, "eventId", env.EventID)

	switch env.EventType {
	case string(events.DeliveryCreated):
		var p events.DeliveryCreatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: %v", pkgKafka.ErrPermanent, err)
		}
		doc := search.DeliveryDocument{
			DeliveryID: p.DeliveryID,
			CustomerID: p.CustomerID,
			DriverID:   p.DriverID,
			Status:     p.Status,
			Pickup: search.GeoAddress{
				City:    p.Pickup.City,
				Country: p.Pickup.Country,
				Location: search.GeoPoint{
					Lat: p.Pickup.Location.Lat,
					Lon: p.Pickup.Location.Lon,
				},
			},
			Dropoff: search.GeoAddress{
				City:    p.Dropoff.City,
				Country: p.Dropoff.Country,
				Location: search.GeoPoint{
					Lat: p.Dropoff.Location.Lat,
					Lon: p.Dropoff.Location.Lon,
				},
			},
			CreatedAt:     p.CreatedAt,
			UpdatedAt:     p.UpdatedAt,
			SourceVersion: p.SourceVersion,
		}
		return cm.indexingService.UpsertDelivery(ctx, doc)

	case string(events.DeliveryDriverAssigned), string(events.DeliveryDriverAccepted),
		string(events.DeliveryPickedUp), string(events.DeliveryInTransit),
		string(events.DeliveryCompleted), string(events.DeliveryCancelled):
		var p events.DeliveryUpdatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: %v", pkgKafka.ErrPermanent, err)
		}
		doc := search.DeliveryDocument{
			DeliveryID:    p.DeliveryID,
			CustomerID:    p.CustomerID,
			DriverID:      p.DriverID,
			Status:        p.Status,
			UpdatedAt:     p.UpdatedAt,
			SourceVersion: p.SourceVersion,
		}
		return cm.indexingService.UpsertDelivery(ctx, doc)

	case string(events.DeliveryDeleted):
		var p events.DeliveryDeletedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: %v", pkgKafka.ErrPermanent, err)
		}
		return cm.indexingService.DeleteDelivery(ctx, p.DeliveryID)

	case string(events.DriverCreated), string(events.DriverUpdated):
		var p events.DriverCreatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: %v", pkgKafka.ErrPermanent, err)
		}
		var geo *search.GeoPoint
		if p.Location != nil {
			geo = &search.GeoPoint{Lat: p.Location.Lat, Lon: p.Location.Lon}
		}
		doc := search.DriverDocument{
			DriverID:      p.DriverID,
			Name:          p.Name,
			Status:        p.Status,
			VehicleType:   p.VehicleType,
			Rating:        p.Rating,
			Location:      geo,
			UpdatedAt:     p.UpdatedAt,
			SourceVersion: p.SourceVersion,
		}
		return cm.indexingService.UpsertDriver(ctx, doc)

	case string(events.DriverDeleted):
		var p events.DriverDeletedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: %v", pkgKafka.ErrPermanent, err)
		}
		return cm.indexingService.DeleteDriver(ctx, p.DriverID)

	case string(events.MediaUploadCreated):
		var p events.MediaUploadCreatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: %v", pkgKafka.ErrPermanent, err)
		}
		doc := search.MediaDocument{
			MediaID:       p.MediaID,
			OwnerID:       p.UserID,
			FileName:      p.FileName,
			MimeType:      p.ContentType,
			MediaType:     p.MediaType,
			Status:        "UPLOADING",
			Size:          p.Size,
			CreatedAt:     time.Now().UTC(),
			SourceVersion: 1,
		}
		return cm.indexingService.UpsertMedia(ctx, doc)

	case string(events.MediaUploadCompleted):
		var p events.MediaUploadCompletedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: %v", pkgKafka.ErrPermanent, err)
		}
		doc := search.MediaDocument{
			MediaID:       p.MediaID,
			OwnerID:       p.UserID,
			FileName:      p.FileName,
			MimeType:      p.ContentType,
			MediaType:     p.MediaType,
			Status:        "UPLOADED",
			Size:          p.Size,
			CreatedAt:     time.Now().UTC(),
			SourceVersion: 1,
		}
		return cm.indexingService.UpsertMedia(ctx, doc)

	case string(events.MediaReady):
		var p events.MediaReadyPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: %v", pkgKafka.ErrPermanent, err)
		}
		size := p.Size
		if size == 0 && len(p.Versions) > 0 {
			size = p.Versions[0].Size
		}
		doc := search.MediaDocument{
			MediaID:       p.MediaID,
			OwnerID:       p.UserID,
			FileName:      p.FileName,
			MediaType:     p.MediaType,
			MimeType:      p.ContentType,
			Status:        "READY",
			Size:          size,
			CreatedAt:     time.Now().UTC(),
			SourceVersion: 1,
		}
		return cm.indexingService.UpsertMedia(ctx, doc)

	case string(events.MediaDeleted):
		var p events.MediaDeletedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: %v", pkgKafka.ErrPermanent, err)
		}
		return cm.indexingService.DeleteMedia(ctx, p.MediaID)

	// User
	case string(events.UserCreated):
		var p events.UserCreatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: invalid user.created payload: %v", pkgKafka.ErrPermanent, err)
		}
		createdAt := p.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		doc := search.UserDocument{
			ID:        p.UserID,
			FirstName: p.FirstName,
			LastName:  p.LastName,
			Email:     p.Email,
			Role:      p.Role,
			IsActive:  true,
			CreatedAt: createdAt,
		}
		return cm.indexingService.UpsertUser(ctx, doc)

	case string(events.UserUpdated):
		var p events.UserUpdatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: invalid user.updated payload: %v", pkgKafka.ErrPermanent, err)
		}
		updatedAt := p.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = time.Now().UTC()
		}
		doc := search.UserDocument{
			ID:        p.UserID,
			FirstName: p.FirstName,
			LastName:  p.LastName,
			Email:     p.Email,
			Role:      p.Role,
			IsActive:  p.IsActive,
			CreatedAt: updatedAt,
		}
		return cm.indexingService.UpsertUser(ctx, doc)

	case string(events.UserDeleted):
		var p events.UserDeletedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: %v", pkgKafka.ErrPermanent, err)
		}
		return cm.indexingService.DeleteUser(ctx, p.UserID)
	}

	return nil
}
