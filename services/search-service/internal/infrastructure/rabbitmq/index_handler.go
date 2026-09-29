package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	shared "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
)

// HandleIndexEvent routes one RabbitMQ envelope to the existing indexing.Service.
// Unknown event types are logged and skipped (nil error → ack, no retry).
// A nil service falls back to log-only mode so the consumer still acks.
func HandleIndexEvent(ctx context.Context, svc *indexing.Service, env *shared.EventEnvelope) error {
	if env == nil {
		return fmt.Errorf("%w: nil envelope", shared.ErrPermanent)
	}
	if svc == nil {
		slog.Info("search RabbitMQ index event (log-only, no indexing service)",
			"eventType", env.EventType, "eventId", env.EventID)
		return nil
	}

	switch env.EventType {
	case string(events.DeliveryCreated):
		var p events.DeliveryCreatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: unmarshal delivery.created: %v", shared.ErrPermanent, err)
		}
		return svc.UpsertDelivery(ctx, search.DeliveryDocument{
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
		})

	case string(events.DeliveryDriverAssigned),
		string(events.DeliveryDriverAccepted),
		string(events.DeliveryPickedUp),
		string(events.DeliveryInTransit),
		string(events.DeliveryCompleted),
		string(events.DeliveryCancelled):
		var p events.DeliveryUpdatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: unmarshal delivery update: %v", shared.ErrPermanent, err)
		}
		return svc.UpsertDelivery(ctx, search.DeliveryDocument{
			DeliveryID:    p.DeliveryID,
			CustomerID:    p.CustomerID,
			DriverID:      p.DriverID,
			Status:        p.Status,
			UpdatedAt:     p.UpdatedAt,
			SourceVersion: p.SourceVersion,
		})

	case string(events.DeliveryDeleted):
		var p events.DeliveryDeletedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: unmarshal delivery.deleted: %v", shared.ErrPermanent, err)
		}
		return svc.DeleteDelivery(ctx, p.DeliveryID)

	case string(events.DriverCreated):
		var p events.DriverCreatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: unmarshal driver.created: %v", shared.ErrPermanent, err)
		}
		var geo *search.GeoPoint
		if p.Location != nil {
			geo = &search.GeoPoint{Lat: p.Location.Lat, Lon: p.Location.Lon}
		}
		return svc.UpsertDriver(ctx, search.DriverDocument{
			DriverID:      p.DriverID,
			Name:          p.Name,
			Status:        p.Status,
			VehicleType:   p.VehicleType,
			Rating:        p.Rating,
			Location:      geo,
			UpdatedAt:     p.UpdatedAt,
			SourceVersion: p.SourceVersion,
		})

	case string(events.DriverUpdated):
		var p events.DriverUpdatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: unmarshal driver.updated: %v", shared.ErrPermanent, err)
		}
		var geo *search.GeoPoint
		if p.Location != nil {
			geo = &search.GeoPoint{Lat: p.Location.Lat, Lon: p.Location.Lon}
		}
		return svc.UpsertDriver(ctx, search.DriverDocument{
			DriverID:      p.DriverID,
			Name:          p.Name,
			Status:        p.Status,
			VehicleType:   p.VehicleType,
			Rating:        p.Rating,
			Location:      geo,
			UpdatedAt:     p.UpdatedAt,
			SourceVersion: p.SourceVersion,
		})

	case string(events.DriverDeleted):
		var p events.DriverDeletedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: unmarshal driver.deleted: %v", shared.ErrPermanent, err)
		}
		return svc.DeleteDriver(ctx, p.DriverID)

	case string(events.MediaUploadCreated):
		var p events.MediaUploadCreatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: unmarshal media.upload.created: %v", shared.ErrPermanent, err)
		}
		return svc.UpsertMedia(ctx, search.MediaDocument{
			MediaID:       p.MediaID,
			OwnerID:       p.UserID,
			FileName:      p.FileName,
			MimeType:      p.ContentType,
			MediaType:     p.MediaType,
			Status:        "UPLOADING",
			Size:          p.Size,
			CreatedAt:     time.Now().UTC(),
			SourceVersion: 1,
		})

	case string(events.MediaUploadCompleted):
		var p events.MediaUploadCompletedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: unmarshal media.upload.completed: %v", shared.ErrPermanent, err)
		}
		return svc.UpsertMedia(ctx, search.MediaDocument{
			MediaID:       p.MediaID,
			OwnerID:       p.UserID,
			FileName:      p.FileName,
			MimeType:      p.ContentType,
			MediaType:     p.MediaType,
			Status:        "UPLOADED",
			Size:          p.Size,
			CreatedAt:     time.Now().UTC(),
			SourceVersion: 1,
		})

	case string(events.MediaReady):
		var p events.MediaReadyPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: unmarshal media.ready: %v", shared.ErrPermanent, err)
		}
		size := p.Size
		if size == 0 && len(p.Versions) > 0 {
			size = p.Versions[0].Size
		}
		return svc.UpsertMedia(ctx, search.MediaDocument{
			MediaID:       p.MediaID,
			OwnerID:       p.UserID,
			FileName:      p.FileName,
			MimeType:      p.ContentType,
			MediaType:     p.MediaType,
			Status:        "READY",
			Size:          size,
			CreatedAt:     time.Now().UTC(),
			SourceVersion: 1,
		})

	case string(events.MediaDeleted):
		var p events.MediaDeletedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: unmarshal media.deleted: %v", shared.ErrPermanent, err)
		}
		return svc.DeleteMedia(ctx, p.MediaID)

	case string(events.UserCreated):
		var p events.UserCreatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: unmarshal user.created: %v", shared.ErrPermanent, err)
		}
		createdAt := p.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		return svc.UpsertUser(ctx, search.UserDocument{
			ID:        p.UserID,
			FirstName: p.FirstName,
			LastName:  p.LastName,
			Email:     p.Email,
			Role:      p.Role,
			IsActive:  true,
			CreatedAt: createdAt,
		})

	case string(events.UserUpdated):
		var p events.UserUpdatedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: unmarshal user.updated: %v", shared.ErrPermanent, err)
		}
		createdAt := p.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		return svc.UpsertUser(ctx, search.UserDocument{
			ID:        p.UserID,
			FirstName: p.FirstName,
			LastName:  p.LastName,
			Email:     p.Email,
			Role:      p.Role,
			IsActive:  p.IsActive,
			CreatedAt: createdAt,
		})

	case string(events.UserDeleted):
		var p events.UserDeletedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("%w: unmarshal user.deleted: %v", shared.ErrPermanent, err)
		}
		return svc.DeleteUser(ctx, p.UserID)

	default:
		slog.Debug("search RabbitMQ: no index handler for event type, skipping",
			"eventType", env.EventType, "eventId", env.EventID)
		return nil
	}
}
