package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
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
	data, err := json.Marshal(map[string]interface{}{
		"type":      eventType,
		"payload":   payload,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("event marshal failed: %w", err)
	}
	return a.pub.PublishMessage(ctx, key, string(data))
}

func (a *EventPublisherAdapter) PublishDriverCreated(ctx context.Context, driverID, userID string) error {
	return a.publish(ctx, "driver.created", driverID, map[string]string{"driverId": driverID, "userId": userID})
}

func (a *EventPublisherAdapter) PublishDriverUpdated(ctx context.Context, driverID string, vehicleType string, capabilities []string) error {
	return a.publish(ctx, "driver.updated", driverID, map[string]interface{}{
		"driverId": driverID, "vehicleType": vehicleType, "capabilities": capabilities,
	})
}

func (a *EventPublisherAdapter) PublishDriverDeleted(ctx context.Context, driverID string) error {
	return a.publish(ctx, "driver.deleted", driverID, map[string]string{"driverId": driverID})
}

func (a *EventPublisherAdapter) PublishAssignmentOffered(ctx context.Context, assignmentID, deliveryID, driverID string) error {
	return a.publish(ctx, "driver.assignment.offered", assignmentID, map[string]string{
		"assignmentId": assignmentID, "deliveryId": deliveryID, "driverId": driverID,
	})
}

func (a *EventPublisherAdapter) PublishAssignmentAccepted(ctx context.Context, assignmentID, deliveryID, driverID string) error {
	return a.publish(ctx, "driver.assignment.accepted", assignmentID, map[string]string{
		"assignmentId": assignmentID, "deliveryId": deliveryID, "driverId": driverID,
		"acceptedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *EventPublisherAdapter) PublishAssignmentRejected(ctx context.Context, assignmentID, deliveryID, driverID, reason string) error {
	return a.publish(ctx, "driver.assignment.rejected", assignmentID, map[string]string{
		"assignmentId": assignmentID, "deliveryId": deliveryID, "driverId": driverID, "reason": reason,
	})
}

func (a *EventPublisherAdapter) PublishAssignmentExpired(ctx context.Context, assignmentID string) error {
	return a.publish(ctx, "driver.assignment.expired", assignmentID, map[string]string{
		"assignmentId": assignmentID, "expiredAt": time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *EventPublisherAdapter) PublishAssignmentReleased(ctx context.Context, assignmentID string) error {
	return a.publish(ctx, "driver.assignment.released", assignmentID, map[string]string{
		"assignmentId": assignmentID, "releasedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *EventPublisherAdapter) PublishDriverAvailable(ctx context.Context, driverID string) error {
	return a.publish(ctx, "driver.available", driverID, map[string]string{"driverId": driverID})
}

func (a *EventPublisherAdapter) PublishDriverUnavailable(ctx context.Context, driverID string) error {
	return a.publish(ctx, "driver.unavailable", driverID, map[string]string{"driverId": driverID})
}

func (a *EventPublisherAdapter) PublishLocationUpdated(ctx context.Context, driverID, latitude, longitude string) error {
	return a.publish(ctx, "driver.location.updated", driverID, map[string]string{
		"driverId": driverID, "latitude": latitude, "longitude": longitude,
	})
}

// Compile-time interface check
var _ ports.EventPublisher = (*EventPublisherAdapter)(nil)
