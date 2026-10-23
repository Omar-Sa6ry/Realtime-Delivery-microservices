package testutil

import (
	"encoding/json"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
)

func NewTestEventEnvelope(eventType, eventID, aggregateID string) *domain.EventEnvelope {
	return &domain.EventEnvelope{
		EventID:       eventID,
		EventType:     eventType,
		EventVersion:  1,
		OccurredAt:    time.Now().UTC(),
		Producer:      "test-service",
		AggregateType: "delivery",
		AggregateID:   aggregateID,
		CorrelationID: "corr-123",
		CausationID:   "cause-456",
		SourceTopic:   "delivery.events",
		SourcePartition: 0,
		SourceOffset:  100,
		IngestedAt:    time.Now().UTC(),
	}
}

func NewTestRawEventLanding(eventType, eventID, aggregateID string) *domain.RawEventLanding {
	return &domain.RawEventLanding{
		EventID:         eventID,
		EventType:       eventType,
		EventVersion:    1,
		AggregateType:   "delivery",
		AggregateID:     aggregateID,
		Producer:        "test-service",
		OccurredAt:      time.Now().UTC(),
		IngestedAt:      time.Now().UTC(),
		CorrelationID:   "corr-123",
		CausationID:     "cause-456",
		SourceTopic:     "delivery.events",
		SourcePartition: 0,
		SourceOffset:    100,
		PayloadJSON:     `{"deliveryId": "` + aggregateID + `"}`,
	}
}

func NewTestFactDeliveryEvent(eventType, eventID, deliveryID string) *domain.FactDeliveryEvent {
	return &domain.FactDeliveryEvent{
		EventID:       eventID,
		DeliveryID:    deliveryID,
		UserID:        "user-123",
		DriverID:      "driver-456",
		EventType:     eventType,
		EventVersion:  1,
		CityID:        "city-1",
		ZoneID:        "zone-1",
		OccurredAt:    time.Now().UTC(),
		IngestedAt:    time.Now().UTC(),
		CorrelationID: "corr-123",
	}
}

func NewTestFactDeliveryCompleted(deliveryID string) *domain.FactDeliveryCompleted {
	now := time.Now().UTC()
	return &domain.FactDeliveryCompleted{
		DeliveryID:  deliveryID,
		UserID:      "user-123",
		DriverID:    "driver-456",
		CreatedAt:   now.Add(-30 * time.Minute),
		AssignedAt:  func() *time.Time { t := now.Add(-25 * time.Minute); return &t }(),
		AcceptedAt:  func() *time.Time { t := now.Add(-24 * time.Minute); return &t }(),
		PickupStartedAt: func() *time.Time { t := now.Add(-20 * time.Minute); return &t }(),
		PickedUpAt:  func() *time.Time { t := now.Add(-18 * time.Minute); return &t }(),
		InTransitAt: func() *time.Time { t := now.Add(-15 * time.Minute); return &t }(),
		DeliveredAt: func() *time.Time { t := now.Add(-5 * time.Minute); return &t }(),
		CompletedAt: func() *time.Time { t := now; return &t }(),
		CityID:      "city-1",
		IngestedAt:  now,
	}
}

func NewTestFactDriverAssignment(assignmentID, deliveryID, driverID string) *domain.FactDriverAssignment {
	return &domain.FactDriverAssignment{
		AssignmentID:   assignmentID,
		DeliveryID:     deliveryID,
		DriverID:       driverID,
		OfferedAt:      time.Now().UTC().Add(-10 * time.Minute),
		DistanceMeters: func() *uint32 { u := uint32(5000); return &u }(),
		Result:         domain.AssignmentAccepted,
		IngestedAt:     time.Now().UTC(),
	}
}

func NewTestFactPaymentTransaction(paymentID, deliveryID string) *domain.FactPaymentTransaction {
	return &domain.FactPaymentTransaction{
		EventID:         "evt-" + paymentID,
		PaymentID:       paymentID,
		DeliveryID:      deliveryID,
		UserID:          "user-123",
		Provider:        "STRIPE",
		TransactionType: domain.TransactionAuthorization,
		Status:          "AUTHORIZED",
		Amount:          "100.50",
		Currency:        "USD",
		ProviderLatencyMs: func() *uint64 { u := uint64(150); return &u }(),
		OccurredAt:      time.Now().UTC(),
		IngestedAt:      time.Now().UTC(),
	}
}

func NewTestFactNotificationEvent(notificationID string) *domain.FactNotificationEvent {
	return &domain.FactNotificationEvent{
		EventID:        "evt-" + notificationID,
		NotificationID: notificationID,
		RecipientType:  "user",
		Channel:        domain.NotificationChannelPush,
		Status:         domain.NotificationStatusSent,
		TemplateID:     "template-123",
		RetryCount:     0,
		Error:          "",
		OccurredAt:     time.Now().UTC(),
		IngestedAt:     time.Now().UTC(),
	}
}

func NewTestDataQualityIssue(eventID, issueType string) *domain.DataQualityIssue {
	return domain.NewDataQualityIssue(
		"issue-"+eventID+":"+issueType,
		eventID,
		issueType,
		"delivery",
		"delivery-123",
		domain.SeverityWarning,
		"test issue",
	)
}

func NewTestIdempotencyRecord(eventID, aggregateType, aggregateID string) *domain.IdempotencyRecord {
	return domain.NewIdempotencyRecord(eventID, aggregateType, aggregateID, "test-topic", 100)
}

func NewTestPayload(mapData map[string]any) ([]byte, error) {
	return json.Marshal(mapData)
}

func NewTestDeliveryCreatedPayload(deliveryID string) ([]byte, error) {
	return NewTestPayload(map[string]any{
		"deliveryId": deliveryID,
		"userId":     "user-123",
		"cityId":     "city-1",
		"zoneId":     "zone-1",
	})
}

func NewTestDeliveryDriverAssignedPayload(deliveryID, driverID string) ([]byte, error) {
	return NewTestPayload(map[string]any{
		"deliveryId":  deliveryID,
		"driverId":    driverID,
		"assignedAt":  time.Now().UTC().Format(time.RFC3339),
	})
}

func NewTestDeliveryCompletedPayload(deliveryID string) ([]byte, error) {
	return NewTestPayload(map[string]any{
		"deliveryId": deliveryID,
		"deliveredAt": time.Now().UTC().Format(time.RFC3339),
		"completedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

func NewTestDriverAssignmentOfferedPayload(assignmentID, deliveryID, driverID string) ([]byte, error) {
	return NewTestPayload(map[string]any{
		"assignmentId":  assignmentID,
		"deliveryId":    deliveryID,
		"driverId":      driverID,
		"offeredAt":     time.Now().UTC().Format(time.RFC3339),
		"distanceMeters": 5000,
	})
}

func NewTestPaymentCreatedPayload(paymentID, deliveryID string) ([]byte, error) {
	return NewTestPayload(map[string]any{
		"paymentId":  paymentID,
		"deliveryId": deliveryID,
		"userId":     "user-123",
		"amount":     "100.50",
		"currency":   "USD",
		"provider":   "STRIPE",
	})
}

func NewTestNotificationCreatedPayload(notificationID string) ([]byte, error) {
	return NewTestPayload(map[string]any{
		"notificationId": notificationID,
		"recipientType":  "user",
		"channel":        "push",
		"templateId":     "template-123",
	})
}

func NewTestEventEnvelopeJSON(env *domain.EventEnvelope, payload []byte) ([]byte, error) {
	return json.Marshal(map[string]any{
		"eventId":        env.EventID,
		"eventType":      env.EventType,
		"eventVersion":   env.EventVersion,
		"occurredAt":     env.OccurredAt.Format(time.RFC3339),
		"producer":       env.Producer,
		"aggregateType":  env.AggregateType,
		"aggregateId":    env.AggregateID,
		"correlationId":  env.CorrelationID,
		"causationId":    env.CausationID,
		"payload":        json.RawMessage(payload),
	})
}