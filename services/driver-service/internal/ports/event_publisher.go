package ports

import (
	"context"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

// EventPublisher defines the interface for publishing events to Kafka and NATS.
type EventPublisher interface {
	PublishDriverCreated(ctx context.Context, driverID, userID string) error
	PublishDriverUpdated(ctx context.Context, driverID string, vehicleType domain.VehicleType, capabilities []string) error
	PublishDriverDeleted(ctx context.Context, driverID string) error
	PublishAssignmentOffered(ctx context.Context, assignmentID, deliveryID, driverID string) error
	PublishAssignmentAccepted(ctx context.Context, assignmentID, deliveryID, driverID string) error
	PublishAssignmentRejected(ctx context.Context, assignmentID, deliveryID, driverID, reason string) error
	PublishAssignmentExpired(ctx context.Context, assignmentID, deliveryID string) error
	PublishAssignmentReleased(ctx context.Context, assignmentID string) error
	PublishDriverAvailable(ctx context.Context, driverID string) error
	PublishDriverUnavailable(ctx context.Context, driverID string) error
	PublishLocationUpdated(ctx context.Context, driverID, latitude, longitude string) error
}