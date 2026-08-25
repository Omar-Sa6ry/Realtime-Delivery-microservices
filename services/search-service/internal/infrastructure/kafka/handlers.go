package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	pkgKafka "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
	"github.com/segmentio/kafka-go"
)

// EventHandler defines the strategy interface for handling specific event types.
type EventHandler interface {
	// EventType returns the event type string this handler processes.
	EventType() string

	// Handle processes the event payload and returns an error if processing fails.
	Handle(ctx context.Context, payload json.RawMessage, traceID string) error
}

// handlerRegistry holds all registered event handlers.
type handlerRegistry struct {
	handlers map[string]EventHandler
}

func newHandlerRegistry() *handlerRegistry {
	return &handlerRegistry{
		handlers: make(map[string]EventHandler),
	}
}

func (r *handlerRegistry) register(h EventHandler) {
	r.handlers[h.EventType()] = h
}

func (r *handlerRegistry) get(eventType string) (EventHandler, bool) {
	h, ok := r.handlers[eventType]
	return h, ok
}

// baseHandler provides common functionality for event handlers.
type baseHandler struct {
	indexingService *indexing.Service
	eventType       string
}

func (b *baseHandler) EventType() string {
	return b.eventType
}

func (b *baseHandler) logInfo(msg string, fields ...interface{}) {
	slog.Info(msg, fields...)
}

func (b *baseHandler) logWarn(msg string, fields ...interface{}) {
	slog.Warn(msg, fields...)
}

// DeliveryCreatedHandler handles delivery.created events.
type DeliveryCreatedHandler struct {
	baseHandler
}

func NewDeliveryCreatedHandler(svc *indexing.Service) *DeliveryCreatedHandler {
	return &DeliveryCreatedHandler{
		baseHandler: baseHandler{
			indexingService: svc,
			eventType:       string(events.DeliveryCreated),
		},
	}
}

func (h *DeliveryCreatedHandler) Handle(ctx context.Context, payload json.RawMessage, traceID string) error {
	var p events.DeliveryCreatedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal delivery.created: %w", err)
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
	return h.indexingService.UpsertDelivery(ctx, doc)
}

// DeliveryUpdatedHandler handles delivery update events (assigned, accepted, picked_up, in_transit, completed, cancelled).
type DeliveryUpdatedHandler struct {
	baseHandler
}

func NewDeliveryUpdatedHandler(svc *indexing.Service) *DeliveryUpdatedHandler {
	return &DeliveryUpdatedHandler{
		baseHandler: baseHandler{
			indexingService: svc,
			eventType:       string(events.DeliveryDriverAssigned), // primary type, will register for others
		},
	}
}

func (h *DeliveryUpdatedHandler) Handle(ctx context.Context, payload json.RawMessage, traceID string) error {
	var p events.DeliveryUpdatedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal delivery updated: %w", err)
	}

	doc := search.DeliveryDocument{
		DeliveryID:    p.DeliveryID,
		CustomerID:    p.CustomerID,
		DriverID:      p.DriverID,
		Status:        p.Status,
		UpdatedAt:     p.UpdatedAt,
		SourceVersion: p.SourceVersion,
	}
	return h.indexingService.UpsertDelivery(ctx, doc)
}

// DeliveryDeletedHandler handles delivery.deleted events.
type DeliveryDeletedHandler struct {
	baseHandler
}

func NewDeliveryDeletedHandler(svc *indexing.Service) *DeliveryDeletedHandler {
	return &DeliveryDeletedHandler{
		baseHandler: baseHandler{
			indexingService: svc,
			eventType:       string(events.DeliveryDeleted),
		},
	}
}

func (h *DeliveryDeletedHandler) Handle(ctx context.Context, payload json.RawMessage, traceID string) error {
	var p events.DeliveryDeletedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal delivery.deleted: %w", err)
	}
	return h.indexingService.DeleteDelivery(ctx, p.DeliveryID)
}

// DriverCreatedHandler handles driver.created and driver.updated events.
type DriverCreatedHandler struct {
	baseHandler
}

func NewDriverCreatedHandler(svc *indexing.Service) *DriverCreatedHandler {
	return &DriverCreatedHandler{
		baseHandler: baseHandler{
			indexingService: svc,
			eventType:       string(events.DriverCreated), // primary type
		},
	}
}

func (h *DriverCreatedHandler) Handle(ctx context.Context, payload json.RawMessage, traceID string) error {
	var p events.DriverCreatedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal driver created/updated: %w", err)
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
	return h.indexingService.UpsertDriver(ctx, doc)
}

// DriverDeletedHandler handles driver.deleted events.
type DriverDeletedHandler struct {
	baseHandler
}

func NewDriverDeletedHandler(svc *indexing.Service) *DriverDeletedHandler {
	return &DriverDeletedHandler{
		baseHandler: baseHandler{
			indexingService: svc,
			eventType:       string(events.DriverDeleted),
		},
	}
}

func (h *DriverDeletedHandler) Handle(ctx context.Context, payload json.RawMessage, traceID string) error {
	var p events.DriverDeletedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal driver.deleted: %w", err)
	}
	return h.indexingService.DeleteDriver(ctx, p.DriverID)
}

// MediaUploadCreatedHandler handles media.upload.created events.
type MediaUploadCreatedHandler struct {
	baseHandler
}

func NewMediaUploadCreatedHandler(svc *indexing.Service) *MediaUploadCreatedHandler {
	return &MediaUploadCreatedHandler{
		baseHandler: baseHandler{
			indexingService: svc,
			eventType:       string(events.MediaUploadCreated),
		},
	}
}

func (h *MediaUploadCreatedHandler) Handle(ctx context.Context, payload json.RawMessage, traceID string) error {
	var p events.MediaUploadCreatedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal media.upload.created: %w", err)
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
	return h.indexingService.UpsertMedia(ctx, doc)
}

// MediaUploadCompletedHandler handles media.upload.completed events.
type MediaUploadCompletedHandler struct {
	baseHandler
}

func NewMediaUploadCompletedHandler(svc *indexing.Service) *MediaUploadCompletedHandler {
	return &MediaUploadCompletedHandler{
		baseHandler: baseHandler{
			indexingService: svc,
			eventType:       string(events.MediaUploadCompleted),
		},
	}
}

func (h *MediaUploadCompletedHandler) Handle(ctx context.Context, payload json.RawMessage, traceID string) error {
	var p events.MediaUploadCompletedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal media.upload.completed: %w", err)
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
	return h.indexingService.UpsertMedia(ctx, doc)
}

// MediaReadyHandler handles media.ready events.
type MediaReadyHandler struct {
	baseHandler
}

func NewMediaReadyHandler(svc *indexing.Service) *MediaReadyHandler {
	return &MediaReadyHandler{
		baseHandler: baseHandler{
			indexingService: svc,
			eventType:       string(events.MediaReady),
		},
	}
}

func (h *MediaReadyHandler) Handle(ctx context.Context, payload json.RawMessage, traceID string) error {
	var p events.MediaReadyPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal media.ready: %w", err)
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
	return h.indexingService.UpsertMedia(ctx, doc)
}

// MediaDeletedHandler handles media.deleted events.
type MediaDeletedHandler struct {
	baseHandler
}

func NewMediaDeletedHandler(svc *indexing.Service) *MediaDeletedHandler {
	return &MediaDeletedHandler{
		baseHandler: baseHandler{
			indexingService: svc,
			eventType:       string(events.MediaDeleted),
		},
	}
}

func (h *MediaDeletedHandler) Handle(ctx context.Context, payload json.RawMessage, traceID string) error {
	var p events.MediaDeletedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal media.deleted: %w", err)
	}
	return h.indexingService.DeleteMedia(ctx, p.MediaID)
}

// UserCreatedHandler handles user.created events.
type UserCreatedHandler struct {
	baseHandler
}

func NewUserCreatedHandler(svc *indexing.Service) *UserCreatedHandler {
	return &UserCreatedHandler{
		baseHandler: baseHandler{
			indexingService: svc,
			eventType:       string(events.UserCreated),
		},
	}
}

func (h *UserCreatedHandler) Handle(ctx context.Context, payload json.RawMessage, traceID string) error {
	var p events.UserCreatedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal user.created: %w", err)
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
	return h.indexingService.UpsertUser(ctx, doc)
}

// UserUpdatedHandler handles user.updated events.
type UserUpdatedHandler struct {
	baseHandler
}

func NewUserUpdatedHandler(svc *indexing.Service) *UserUpdatedHandler {
	return &UserUpdatedHandler{
		baseHandler: baseHandler{
			indexingService: svc,
			eventType:       string(events.UserUpdated),
		},
	}
}

func (h *UserUpdatedHandler) Handle(ctx context.Context, payload json.RawMessage, traceID string) error {
	var p events.UserUpdatedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal user.updated: %w", err)
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
	return h.indexingService.UpsertUser(ctx, doc)
}

// UserDeletedHandler handles user.deleted events.
type UserDeletedHandler struct {
	baseHandler
}

func NewUserDeletedHandler(svc *indexing.Service) *UserDeletedHandler {
	return &UserDeletedHandler{
		baseHandler: baseHandler{
			indexingService: svc,
			eventType:       string(events.UserDeleted),
		},
	}
}

func (h *UserDeletedHandler) Handle(ctx context.Context, payload json.RawMessage, traceID string) error {
	var p events.UserDeletedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal user.deleted: %w", err)
	}
	return h.indexingService.DeleteUser(ctx, p.UserID)
}

// BuildRegistry creates and populates the handler registry with all event handlers.
func BuildRegistry(svc *indexing.Service) *handlerRegistry {
	registry := newHandlerRegistry()

	// Delivery events
	deliveryCreated := NewDeliveryCreatedHandler(svc)
	deliveryUpdated := NewDeliveryUpdatedHandler(svc)
	deliveryDeleted := NewDeliveryDeletedHandler(svc)

	registry.register(deliveryCreated)
	// Register the same handler for multiple delivery update event types
	registry.registerForTypes(deliveryUpdated, []string{
		string(events.DeliveryDriverAssigned),
		string(events.DeliveryDriverAccepted),
		string(events.DeliveryPickedUp),
		string(events.DeliveryInTransit),
		string(events.DeliveryCompleted),
		string(events.DeliveryCancelled),
	})
	registry.register(deliveryDeleted)

	// Driver events
	driverCreated := NewDriverCreatedHandler(svc)
	driverDeleted := NewDriverDeletedHandler(svc)

	registry.register(driverCreated)
	// Also handle driver.updated with the same handler
	registry.registerForTypes(driverCreated, []string{
		string(events.DriverUpdated),
	})
	registry.register(driverDeleted)

	// Media events
	registry.register(NewMediaUploadCreatedHandler(svc))
	registry.register(NewMediaUploadCompletedHandler(svc))
	registry.register(NewMediaReadyHandler(svc))
	registry.register(NewMediaDeletedHandler(svc))

	// User events
	registry.register(NewUserCreatedHandler(svc))
	registry.register(NewUserUpdatedHandler(svc))
	registry.register(NewUserDeletedHandler(svc))

	return registry
}

// registerForTypes registers a single handler for multiple event types.
func (r *handlerRegistry) registerForTypes(h EventHandler, types []string) {
	for _, t := range types {
		r.handlers[t] = h
	}
}

// HandleEvent dispatches the event to the appropriate handler using the registry.
func (cm *ConsumerManager) HandleEvent(ctx context.Context, registry *handlerRegistry, msg kafka.Message) error {
	env, err := events.UnmarshalEnvelope(msg.Value)
	if err != nil {
		return fmt.Errorf("%w: invalid envelope json: %v", pkgKafka.ErrPermanent, err)
	}

	slog.Info("Consuming event for search projection", "topic", msg.Topic, "eventType", env.EventType, "eventId", env.EventID)

	handler, ok := registry.get(env.EventType)
	if !ok {
		slog.Debug("No handler for event type, skipping", "eventType", env.EventType)
		return nil
	}

	return handler.Handle(ctx, env.Payload, env.TraceID)
}