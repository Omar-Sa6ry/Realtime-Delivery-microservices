package ports

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

type OutboxEvent struct {
	ID        string
	EventType string
	Payload   []byte
	CreatedAt int64
}
type PaymentRepository interface {
	Create(ctx context.Context, p *domain.Payment) error
	FindByID(ctx context.Context, id string) (*domain.Payment, error)
	FindByDeliveryID(ctx context.Context, deliveryID string) (*domain.Payment, error)
	FindByUserID(ctx context.Context, userID string) ([]*domain.Payment, error)
	FindByIDs(ctx context.Context, ids []string) ([]*domain.Payment, error)
	FindByProviderPaymentID(ctx context.Context, providerPaymentID string) (*domain.Payment, error)
	// Returns ErrConcurrentModification if the version does not match.
	UpdateConditional(ctx context.Context, p *domain.Payment, expectedVersion int64) error
}

type AttemptRepository interface {
	Create(ctx context.Context, a *domain.Attempt) error
	FindByPaymentID(ctx context.Context, paymentID string) ([]*domain.Attempt, error)
	UpdateStatus(ctx context.Context, id string, status domain.OperationStatus, providerTxID string) error
	FindStuck(ctx context.Context, thresholdSeconds int) ([]*domain.Attempt, error)
	FindUnknown(ctx context.Context, limit int) ([]*domain.Attempt, error)
}

type RefundRepository interface {
	Create(ctx context.Context, r *domain.Refund) error
	FindByID(ctx context.Context, id string) (*domain.Refund, error)
	FindByPaymentID(ctx context.Context, paymentID string) ([]*domain.Refund, error)
	UpdateStatus(ctx context.Context, id string, status domain.RefundStatus, providerRefundID string) error
}

type OutboxRepository interface {
	Insert(ctx context.Context, eventType string, payload []byte) error
	FetchUnpublished(ctx context.Context, limit int) ([]*OutboxEvent, error)
	MarkPublished(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, reason string) error
	Cleanup(ctx context.Context, olderThanDays int) (int64, error)
}

type IdempotencyStore interface {
	CheckOrCreate(ctx context.Context, key, operation, requestHash string) (*IdempotencyRecord, bool, error)
	Complete(ctx context.Context, key string, responsePayload []byte) error
	Cleanup(ctx context.Context) error
}

type IdempotencyRecord struct {
	Key             string
	Operation       string
	RequestHash     string
	ResponsePayload []byte
	CreatedAt       int64
	CompletedAt     *int64
}

type AuditLogRepository interface {
	Log(ctx context.Context, paymentID, operation, actor, details string) error
}

type EventPublisher interface {
	PublishEvent(ctx context.Context, topic, key, eventType string, payload []byte) error
}

type RealtimePublisher interface {
	PublishPaymentStatusUpdated(ctx context.Context, payload []byte) error
}
