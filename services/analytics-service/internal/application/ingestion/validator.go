package ingestion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/ingestion/handlers"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
)

type rawEnvelope struct {
	EventID       string          `json:"eventId"`
	EventType     string          `json:"eventType"`
	EventVersion  int             `json:"eventVersion"`
	OccurredAt    json.RawMessage `json:"occurredAt"`
	Producer      string          `json:"producer"`
	AggregateType string          `json:"aggregateType"`
	AggregateID   string          `json:"aggregateId"`
	CorrelationID string          `json:"correlationId"`
	CausationID   string          `json:"causationId"`
	Payload       json.RawMessage `json:"payload"`
}

func DecodeEnvelope(data []byte) (*domain.EventEnvelope, handlers.Payload, error) {
	var raw rawEnvelope
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, handlers.Payload{}, fmt.Errorf("%w: decode: %v", domain.ErrInvalidEnvelope, err)
	}
	occurredAt, err := parseFlexTime(raw.OccurredAt)
	if err != nil {
		return nil, handlers.Payload{}, fmt.Errorf("%w: occurredAt: %v", domain.ErrInvalidOccurredAt, err)
	}
	payload := handlers.Payload{Raw: raw.Payload}
	if len(bytes.TrimSpace(raw.Payload)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(raw.Payload))
		dec.UseNumber()
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			return nil, handlers.Payload{}, fmt.Errorf("%w: payload: %v", domain.ErrInvalidEnvelope, err)
		}
		payload.Map = m
	} else {
		payload.Map = map[string]any{}
	}
	env := &domain.EventEnvelope{
		EventID:       raw.EventID,
		EventType:     raw.EventType,
		EventVersion:  domain.EventVersion(raw.EventVersion),
		OccurredAt:    occurredAt,
		Producer:      raw.Producer,
		AggregateType: raw.AggregateType,
		AggregateID:   raw.AggregateID,
		CorrelationID: raw.CorrelationID,
		CausationID:   raw.CausationID,
	}
	return env, payload, nil
}

func parseFlexTime(raw json.RawMessage) (time.Time, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return time.Time{}, fmt.Errorf("missing")
	}
	if strings.HasPrefix(s, `"`) {
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return time.Time{}, err
		}
		return time.Parse(time.RFC3339, strings.TrimSpace(str))
	}
	var num json.Number
	if err := json.Unmarshal(raw, &num); err != nil {
		return time.Time{}, err
	}
	ms, err := num.Int64()
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(ms).UTC(), nil
}

type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) ValidateEnvelope(env *domain.EventEnvelope) error {
	return env.Validate()
}

var requiredPayloadFields = map[string][]string{
	// Delivery domain.
	"delivery.created":         {"deliveryId"},
	"delivery.driver.assigned": {"deliveryId", "driverId"},
	"delivery.driver.accepted": {"deliveryId", "driverId"},
	"delivery.pickup.started":  {"deliveryId"},
	"delivery.picked_up":       {"deliveryId"},
	"delivery.in_transit":      {"deliveryId"},
	"delivery.completed":       {"deliveryId"},
	"delivery.cancelled":       {"deliveryId"},
	"delivery.failed":          {"deliveryId"},
	// Driver domain.
	"driver.available":           {"driverId"},
	"driver.unavailable":         {"driverId"},
	"driver.assignment.offered":  {"assignmentId", "driverId"},
	"driver.assignment.accepted": {"assignmentId"},
	"driver.assignment.rejected": {"assignmentId"},
	"driver.assignment.expired":  {"assignmentId"},
	"driver.assignment.released": {"assignmentId"},
	// Payment domain.
	"payment.created":               {"paymentId"},
	"payment.authorization.started": {"paymentId"},
	"payment.authorized":            {"paymentId"},
	"payment.authorization.failed":  {"paymentId"},
	"payment.capture.started":       {"paymentId"},
	"payment.captured":              {"paymentId"},
	"payment.capture.failed":        {"paymentId"},
	"payment.cancelled":             {"paymentId"},
	"payment.refund.started":        {"paymentId"},
	"payment.refunded":              {"paymentId"},
	"payment.refund.failed":         {"paymentId"},
	"payment.failed":                {"paymentId"},
	// Notification domain.
	"notification.created":   {"notificationId"},
	"notification.sent":      {"notificationId"},
	"notification.delivered": {"notificationId"},
	"notification.failed":    {"notificationId"},
	"notification.retrying":  {"notificationId"},
}

func (v *Validator) ValidateSchema(env *domain.EventEnvelope, payload handlers.Payload) error {
	required, ok := requiredPayloadFields[env.EventType]
	if !ok {
		return fmt.Errorf("%w: %s", domain.ErrUnknownEventType, env.EventType)
	}
	for _, key := range required {
		val, present := payload.Map[key]
		if !present || val == nil {
			return fmt.Errorf("%w: %s", domain.ErrInvalidEnvelope, key)
		}
		if s, isStr := val.(string); isStr && strings.TrimSpace(s) == "" {
			return fmt.Errorf("%w: %s", domain.ErrInvalidEnvelope, key)
		}
	}
	return nil
}
