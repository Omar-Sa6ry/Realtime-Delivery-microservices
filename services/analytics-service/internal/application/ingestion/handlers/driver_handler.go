package handlers

import (
	"context"
	"fmt"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
)

// driverAssignmentEvents are the assignment lifecycle events with fact rows.
// Availability signals (driver.available/unavailable) carry no measurable
// assignment and land as raw rows only — current availability stays owned
// by Driver & Dispatch.
var driverAssignmentEvents = map[string]domain.AssignmentResult{
	"driver.assignment.offered":  domain.AssignmentOffered,
	"driver.assignment.accepted": domain.AssignmentAccepted,
	"driver.assignment.rejected": domain.AssignmentRejected,
	"driver.assignment.expired":  domain.AssignmentExpired,
	"driver.assignment.released": domain.AssignmentReleased,
}

// DriverHandler transforms driver.* events into assignment facts.
type DriverHandler struct {
	BaseHandler
}

// Handles reports whether the full event type belongs to this handler.
func (h *DriverHandler) Handles(eventType string) bool {
	if _, ok := driverAssignmentEvents[eventType]; ok {
		return true
	}
	return eventType == "driver.available" || eventType == "driver.unavailable"
}

// Handle builds the assignment fact (or raw-only for availability signals).
func (h *DriverHandler) Handle(ctx context.Context, env *domain.EventEnvelope, payload Payload) (*FactBatch, error) {
	_ = ctx
	batch := &FactBatch{}
	raw := h.NewRawLanding(env)
	raw.PayloadJSON = string(payload.Raw)
	batch.Raw = append(batch.Raw, raw)

	result, isAssignment := driverAssignmentEvents[env.EventType]
	if !isAssignment {
		batch.Seen = append(batch.Seen, h.NewSeen(env))
		return batch, nil
	}

	m := payload.Map
	assignmentID := firstNonEmpty(GetString(m, "assignmentId"), env.AggregateID)
	if assignmentID == "" {
		return nil, fmt.Errorf("%w: assignmentId", domain.ErrInvalidEnvelope)
	}
	fact := &domain.FactDriverAssignment{
		AssignmentID:   assignmentID,
		DeliveryID:     GetString(m, "deliveryId"),
		DriverID:       firstNonEmpty(GetString(m, "driverId")),
		OfferedAt:      GetTime(m, "offeredAt", env.OccurredAt),
		DistanceMeters: GetUint32(m, "distanceMeters"),
		Result:         result,
		IngestedAt:     env.IngestedAt,
	}
	respondedAt := GetTime(m, "respondedAt", env.OccurredAt)
	switch result {
	case domain.AssignmentAccepted:
		fact.AcceptedAt = &respondedAt
	case domain.AssignmentRejected:
		fact.RejectedAt = &respondedAt
	case domain.AssignmentExpired:
		fact.ExpiredAt = &respondedAt
	case domain.AssignmentReleased:
		fact.ReleasedAt = &respondedAt
	}
	fact.ComputeResponseTime()
	if err := fact.Validate(); err != nil {
		return nil, err
	}
	batch.DriverAssignments = append(batch.DriverAssignments, fact)
	batch.Seen = append(batch.Seen, h.NewSeen(env))
	return batch, nil
}
