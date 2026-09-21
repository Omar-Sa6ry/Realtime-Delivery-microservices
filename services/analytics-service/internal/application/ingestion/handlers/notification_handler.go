package handlers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
)

var notificationStatusMapping = map[string]domain.NotificationStatus{
	"notification.created":   domain.NotificationStatusCreated,
	"notification.sent":      domain.NotificationStatusSent,
	"notification.delivered": domain.NotificationStatusDelivered,
	"notification.failed":    domain.NotificationStatusFailed,
	"notification.retrying":  domain.NotificationStatusRetrying,
}

type NotificationHandler struct {
	BaseHandler
}

func (h *NotificationHandler) Handles(eventType string) bool {
	_, ok := notificationStatusMapping[eventType]
	return ok
}

func (h *NotificationHandler) Handle(ctx context.Context, env *domain.EventEnvelope, payload Payload) (*FactBatch, error) {
	_ = ctx
	m := payload.Map
	notificationID := firstNonEmpty(GetString(m, "notificationId"), env.AggregateID)
	if notificationID == "" {
		return nil, fmt.Errorf("%w: notificationId", domain.ErrInvalidEnvelope)
	}
	status, ok := notificationStatusMapping[env.EventType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", domain.ErrUnknownEventType, env.EventType)
	}
	channel := domain.NotificationChannel(GetString(m, "channel"))
	if channel == "" {
		channel = domain.NotificationChannelPush
	}
	fact := &domain.FactNotificationEvent{
		EventID:        env.EventID,
		NotificationID: notificationID,
		RecipientType:  GetString(m, "recipientType"),
		Channel:        channel,
		Status:         status,
		TemplateID:     GetString(m, "templateId"),
		RetryCount:     getRetryCount(m),
		Error:          GetString(m, "error"),
		OccurredAt:     env.OccurredAt,
		IngestedAt:     env.IngestedAt,
	}
	if v, ok := m["sentAt"]; ok && v != nil {
		tm := GetTime(m, "sentAt", env.OccurredAt)
		fact.SentAt = &tm
	}
	if v, ok := m["deliveredAt"]; ok && v != nil {
		tm := GetTime(m, "deliveredAt", env.OccurredAt)
		fact.DeliveredAt = &tm
	}
	if v, ok := m["failedAt"]; ok && v != nil {
		tm := GetTime(m, "failedAt", env.OccurredAt)
		fact.FailedAt = &tm
	}
	if err := fact.Validate(); err != nil {
		return nil, err
	}
	batch := &FactBatch{}
	raw := h.NewRawLanding(env)
	raw.PayloadJSON = string(payload.Raw)
	batch.Raw = append(batch.Raw, raw)
	batch.NotificationEvents = append(batch.NotificationEvents, fact)
	batch.Seen = append(batch.Seen, h.NewSeen(env))
	return batch, nil
}

func getRetryCount(m map[string]any) int {
	v, ok := m["retryCount"]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		if t < 0 {
			return 0
		}
		return int(t)
	default:
		if s := GetString(m, "retryCount"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n >= 0 {
				return n
			}
		}
		return 0
	}
}
