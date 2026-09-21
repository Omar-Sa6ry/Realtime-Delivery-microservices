package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
)

type DeliveryHandler struct {
	BaseHandler
}

func (h *DeliveryHandler) Handles(eventType string) bool {
	return domain.KnownDeliveryEventTypes[eventType]
}

// Handle builds the immutable event row plus the denormalized timeline row
// carrying this event's observation. Full cross-event timeline assembly uses
// MergeTimeline once a loader is wired; the single-event row is always safe
// to write because ReplacingMergeTree keeps the latest ingested state.
func (h *DeliveryHandler) Handle(ctx context.Context, env *domain.EventEnvelope, payload Payload) (*FactBatch, error) {
	_ = ctx
	m := payload.Map
	deliveryID := firstNonEmpty(GetString(m, "deliveryId"), env.AggregateID)
	if deliveryID == "" {
		return nil, fmt.Errorf("%w: deliveryId", domain.ErrInvalidEnvelope)
	}

	event := &domain.FactDeliveryEvent{
		EventID:       env.EventID,
		DeliveryID:    deliveryID,
		UserID:        GetString(m, "userId"),
		DriverID:      GetString(m, "driverId"),
		EventType:     env.EventType,
		EventVersion:  env.EventVersion,
		CityID:        GetString(m, "cityId"),
		ZoneID:        GetString(m, "zoneId"),
		OccurredAt:    env.OccurredAt,
		IngestedAt:    env.IngestedAt,
		CorrelationID: env.CorrelationID,
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}

	batch := &FactBatch{}
	raw := h.NewRawLanding(env)
	raw.PayloadJSON = string(payload.Raw)
	batch.Raw = append(batch.Raw, raw)
	batch.DeliveryEvents = append(batch.DeliveryEvents, event)

	timeline := MergeTimeline(nil, event, m)
	if err := timeline.Validate(); err != nil {
		batch.DataQuality = append(batch.DataQuality,
			h.Qualify(env, domain.IssueOutOfOrderTimestamps, domain.SeverityWarning, err.Error()))
	} else if err := timeline.ComputeDurations(); err != nil {
		batch.DataQuality = append(batch.DataQuality,
			h.Qualify(env, domain.IssueNegativeDuration, domain.SeverityError, err.Error()))
	} else {
		batch.DeliveryCompleted = append(batch.DeliveryCompleted, timeline)
	}
	batch.Seen = append(batch.Seen, h.NewSeen(env))
	return batch, nil
}

// MergeTimeline folds one delivery event into a timeline row, starting from an
// existing row when a loader provides it (nil builds from this event alone).
// Pure function: no I/O, fully unit-testable.
func MergeTimeline(existing *domain.FactDeliveryCompleted, event *domain.FactDeliveryEvent, payload map[string]any) *domain.FactDeliveryCompleted {
	var t domain.FactDeliveryCompleted
	if existing != nil {
		t = *existing
	} else {
		t = domain.FactDeliveryCompleted{
			DeliveryID: event.DeliveryID,
			UserID:     event.UserID,
			DriverID:   event.DriverID,
			CreatedAt:  event.OccurredAt,
			CityID:     event.CityID,
		}
	}
	if event.UserID != "" {
		t.UserID = event.UserID
	}
	if event.DriverID != "" {
		t.DriverID = event.DriverID
	}
	if event.CityID != "" {
		t.CityID = event.CityID
	}
	at := func(keys ...string) *time.Time {
		for _, k := range keys {
			if _, ok := payload[k]; ok {
				tm := GetTime(payload, k, time.Time{})
				if !tm.IsZero() {
					return &tm
				}
			}
		}
		tm := event.OccurredAt
		return &tm
	}
	switch event.EventType {
	case domain.DeliveryCreated:
		if t.CreatedAt.IsZero() {
			t.CreatedAt = event.OccurredAt
		}
	case domain.DeliveryDriverAssigned:
		t.AssignedAt = at("assignedAt")
	case domain.DeliveryDriverAccepted:
		t.AcceptedAt = at("acceptedAt")
	case domain.DeliveryPickupStarted:
		t.PickupStartedAt = at("pickupStartedAt")
	case domain.DeliveryPickedUp:
		t.PickedUpAt = at("pickedUpAt")
	case domain.DeliveryInTransit:
		t.InTransitAt = at("inTransitAt")
	case domain.DeliveryCompleted:
		t.DeliveredAt = at("deliveredAt")
		t.CompletedAt = at("completedAt")
	}
	t.IngestedAt = event.IngestedAt
	return &t
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
