package kafka

import (
	"context"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
	"github.com/google/uuid"
)

// EventPublisherAdapter wraps KafkaPublisher and implements ports.EventPublisher.
type EventPublisherAdapter struct {
	pub *KafkaPublisher
}

// NewEventPublisherAdapter creates an EventPublisherAdapter that satisfies ports.EventPublisher.
func NewEventPublisherAdapter(pub *KafkaPublisher) *EventPublisherAdapter {
	return &EventPublisherAdapter{pub: pub}
}

func (a *EventPublisherAdapter) publish(ctx context.Context, eventType, key string, payload interface{}) error {
	eventID := uuid.New().String()
	data, err := events.MarshalEnvelope(eventID, eventType, "", payload)
	if err != nil {
		return err
	}
	return a.pub.PublishMessage(ctx, eventType, key, data)
}

func (a *EventPublisherAdapter) PublishDriverCreated(ctx context.Context, driverID, userID string) error {
	payload := events.DriverCreatedPayload{
		DriverID:      driverID,
		Name:          userID, // User identifier/name
		Status:        "AVAILABLE",
		VehicleType:   "CAR",
		Rating:        5.0,
		UpdatedAt:     time.Now().UTC(),
		SourceVersion: 1,
	}
	return a.publish(ctx, string(events.DriverCreated), driverID, payload)
}

func (a *EventPublisherAdapter) PublishDriverUpdated(ctx context.Context, driverID string, vehicleType domain.VehicleType, capabilities []string) error {
	payload := events.DriverUpdatedPayload{
		DriverID:      driverID,
		VehicleType:   string(vehicleType),
		UpdatedAt:     time.Now().UTC(),
		SourceVersion: 1,
	}
	return a.publish(ctx, string(events.DriverUpdated), driverID, payload)
}

func (a *EventPublisherAdapter) PublishDriverDeleted(ctx context.Context, driverID string) error {
	payload := events.DriverDeletedPayload{
		DriverID:  driverID,
		DeletedAt: time.Now().UTC(),
	}
	return a.publish(ctx, string(events.DriverDeleted), driverID, payload)
}

func (a *EventPublisherAdapter) PublishAssignmentOffered(ctx context.Context, assignmentID, deliveryID, driverID string) error {
	payload := events.DriverAssignmentOfferedPayload{
		AssignmentID: assignmentID,
		DeliveryID:   deliveryID,
		DriverID:     driverID,
		ExpiresAt:    time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339),
	}
	return a.publish(ctx, string(events.DriverAssignmentOffered), assignmentID, payload)
}

func (a *EventPublisherAdapter) PublishAssignmentAccepted(ctx context.Context, assignmentID, deliveryID, driverID string) error {
	payload := events.DriverAssignmentAcceptedPayload{
		AssignmentID: assignmentID,
		DeliveryID:   deliveryID,
		DriverID:     driverID,
		AcceptedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	return a.publish(ctx, string(events.DriverAssignmentAccepted), assignmentID, payload)
}

func (a *EventPublisherAdapter) PublishAssignmentRejected(ctx context.Context, assignmentID, deliveryID, driverID, reason string) error {
	payload := events.DriverAssignmentRejectedPayload{
		AssignmentID: assignmentID,
		DeliveryID:   deliveryID,
		DriverID:     driverID,
		Reason:       reason,
	}
	return a.publish(ctx, string(events.DriverAssignmentRejected), assignmentID, payload)
}

func (a *EventPublisherAdapter) PublishAssignmentExpired(ctx context.Context, assignmentID, deliveryID string) error {
	payload := events.DriverAssignmentExpiredPayload{
		AssignmentID: assignmentID,
		DeliveryID:   deliveryID,
		ExpiredAt:    time.Now().UTC().Format(time.RFC3339),
	}
	return a.publish(ctx, string(events.DriverAssignmentExpired), assignmentID, payload)
}

func (a *EventPublisherAdapter) PublishAssignmentReleased(ctx context.Context, assignmentID string) error {
	payload := events.DriverAssignmentReleasedPayload{
		AssignmentID: assignmentID,
		ReleasedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	return a.publish(ctx, string(events.DriverAssignmentReleased), assignmentID, payload)
}

func (a *EventPublisherAdapter) PublishDriverAvailable(ctx context.Context, driverID string) error {
	payload := events.DriverAvailablePayload{
		DriverID: driverID,
	}
	return a.publish(ctx, string(events.DriverAvailable), driverID, payload)
}

func (a *EventPublisherAdapter) PublishDriverUnavailable(ctx context.Context, driverID string) error {
	payload := events.DriverUnavailablePayload{
		DriverID: driverID,
	}
	return a.publish(ctx, string(events.DriverUnavailable), driverID, payload)
}

func (a *EventPublisherAdapter) PublishLocationUpdated(ctx context.Context, driverID, latitude, longitude string) error {
	return a.publish(ctx, "driver.location.updated", driverID, map[string]string{
		"driverId": driverID, "latitude": latitude, "longitude": longitude,
	})
}

// Compile-time interface check
var _ ports.EventPublisher = (*EventPublisherAdapter)(nil)

