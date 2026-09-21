package handlers

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
)

type Payload struct {
	Map map[string]any
	Raw json.RawMessage
}

type FactBatch struct {
	Raw                 []*domain.RawEventLanding
	DeliveryEvents      []*domain.FactDeliveryEvent
	DeliveryCompleted   []*domain.FactDeliveryCompleted
	DriverAssignments   []*domain.FactDriverAssignment
	PaymentTransactions []*domain.FactPaymentTransaction
	NotificationEvents  []*domain.FactNotificationEvent
	DataQuality         []*domain.DataQualityIssue
	Seen                []*domain.IdempotencyRecord
}

func (b *FactBatch) RowCount() int {
	if b == nil {
		return 0
	}
	return len(b.Raw) + len(b.DeliveryEvents) + len(b.DeliveryCompleted) +
		len(b.DriverAssignments) + len(b.PaymentTransactions) +
		len(b.NotificationEvents) + len(b.DataQuality)
}

type Handler interface {
	Handles(eventType string) bool
	Handle(ctx context.Context, env *domain.EventEnvelope, payload Payload) (*FactBatch, error)
}

type BaseHandler struct{}

func (BaseHandler) NewRawLanding(env *domain.EventEnvelope) *domain.RawEventLanding {
	return &domain.RawEventLanding{
		EventID:         env.EventID,
		EventType:       env.EventType,
		EventVersion:    env.EventVersion,
		AggregateType:   env.AggregateType,
		AggregateID:     env.AggregateID,
		Producer:        env.Producer,
		OccurredAt:      env.OccurredAt,
		IngestedAt:      env.IngestedAt,
		CorrelationID:   env.CorrelationID,
		CausationID:     env.CausationID,
		SourceTopic:     env.SourceTopic,
		SourcePartition: env.SourcePartition,
		SourceOffset:    env.SourceOffset,
	}
}

func (BaseHandler) NewSeen(env *domain.EventEnvelope) *domain.IdempotencyRecord {
	return domain.NewIdempotencyRecord(env.EventID, env.AggregateType, env.AggregateID, env.SourceTopic, env.SourceOffset)
}

func (BaseHandler) Qualify(env *domain.EventEnvelope, issueType string, severity domain.DataQualitySeverity, details string) *domain.DataQualityIssue {
	return domain.NewDataQualityIssue(
		env.EventID+":"+issueType,
		env.EventID, issueType, env.AggregateType, env.AggregateID,
		severity, details,
	)
}

func GetString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}

func GetTime(m map[string]any, key string, fallback time.Time) time.Time {
	v, ok := m[key]
	if !ok || v == nil {
		return fallback
	}
	switch t := v.(type) {
	case string:
		if tm, err := time.Parse(time.RFC3339, strings.TrimSpace(t)); err == nil {
			return tm
		}
		return fallback
	case json.Number:
		if ms, err := t.Int64(); err == nil {
			return time.UnixMilli(ms).UTC()
		}
		return fallback
	case float64:
		return time.UnixMilli(int64(t)).UTC()
	default:
		return fallback
	}
}

func GetUint32(m map[string]any, key string) *uint32 {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	var n int64
	switch t := v.(type) {
	case json.Number:
		parsed, err := t.Int64()
		if err != nil || parsed < 0 {
			return nil
		}
		n = parsed
	case float64:
		if t < 0 {
			return nil
		}
		n = int64(t)
	default:
		return nil
	}
	u := uint32(n)
	return &u
}

func GetUint64(m map[string]any, key string) *uint64 {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	var n uint64
	switch t := v.(type) {
	case json.Number:
		parsed, err := strconv.ParseUint(t.String(), 10, 64)
		if err != nil {
			return nil
		}
		n = parsed
	case float64:
		if t < 0 {
			return nil
		}
		n = uint64(t)
	default:
		return nil
	}
	return &n
}
