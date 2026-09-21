package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
)

var paymentTypeMapping = map[string]domain.PaymentTransactionType{
	"payment.created":               domain.TransactionAuthorization,
	"payment.authorization.started": domain.TransactionAuthorization,
	"payment.authorized":            domain.TransactionAuthorization,
	"payment.authorization.failed":  domain.TransactionAuthorization,
	"payment.capture.started":       domain.TransactionCapture,
	"payment.captured":              domain.TransactionCapture,
	"payment.capture.failed":        domain.TransactionCapture,
	"payment.cancelled":             domain.TransactionCancel,
	"payment.refund.started":        domain.TransactionRefund,
	"payment.refunded":              domain.TransactionRefund,
	"payment.refund.failed":         domain.TransactionRefund,
	"payment.failed":                domain.TransactionAuthorization,
}

type PaymentHandler struct {
	BaseHandler
}

func (h *PaymentHandler) Handles(eventType string) bool {
	_, ok := paymentTypeMapping[eventType]
	return ok
}

func (h *PaymentHandler) Handle(ctx context.Context, env *domain.EventEnvelope, payload Payload) (*FactBatch, error) {
	_ = ctx
	m := payload.Map
	paymentID := firstNonEmpty(GetString(m, "paymentId"), env.AggregateID)
	if paymentID == "" {
		return nil, fmt.Errorf("%w: paymentId", domain.ErrInvalidEnvelope)
	}
	txType, ok := paymentTypeMapping[env.EventType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", domain.ErrUnknownEventType, env.EventType)
	}
	fact := &domain.FactPaymentTransaction{
		EventID:           env.EventID,
		PaymentID:         paymentID,
		DeliveryID:        GetString(m, "deliveryId"),
		UserID:            GetString(m, "userId"),
		Provider:          GetString(m, "provider"),
		TransactionType:   txType,
		Status:            derivePaymentStatus(env.EventType, m),
		Amount:            GetString(m, "amount"),
		Currency:          GetString(m, "currency"),
		ProviderLatencyMs: GetUint64(m, "providerLatencyMs"),
		OccurredAt:        env.OccurredAt,
		IngestedAt:        env.IngestedAt,
	}
	if err := fact.Validate(); err != nil {
		return nil, err
	}
	batch := &FactBatch{}
	raw := h.NewRawLanding(env)
	raw.PayloadJSON = string(payload.Raw)
	batch.Raw = append(batch.Raw, raw)
	batch.PaymentTransactions = append(batch.PaymentTransactions, fact)
	batch.Seen = append(batch.Seen, h.NewSeen(env))
	return batch, nil
}

func derivePaymentStatus(eventType string, m map[string]any) string {
	if s := GetString(m, "status"); s != "" {
		return strings.ToUpper(s)
	}
	switch {
	case strings.HasSuffix(eventType, ".started"):
		return "PENDING"
	case strings.HasSuffix(eventType, ".failed"):
		return "FAILED"
	case strings.HasSuffix(eventType, ".cancelled"):
		return "CANCELLED"
	case strings.HasSuffix(eventType, ".authorized"):
		return "AUTHORIZED"
	case strings.HasSuffix(eventType, ".captured"):
		return "CAPTURED"
	case strings.HasSuffix(eventType, ".refunded"):
		return "REFUNDED"
	default:
		return "UNKNOWN"
	}
}
