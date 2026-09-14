package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

type RefundRepository struct {
	db *sql.DB
}

func NewRefundRepository(db *sql.DB) *RefundRepository {
	return &RefundRepository{db: db}
}

func (r *RefundRepository) Create(ctx context.Context, refund *domain.Refund) error {
	query := `INSERT INTO refunds
		(id, payment_id, delivery_id, amount_minor, currency, status, reason,
		 provider_refund_id, idempotency_key, correlation_id, causation_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	_, err := r.db.ExecContext(ctx, query,
		refund.ID, refund.PaymentID, refund.DeliveryID,
		refund.AmountMinor, refund.Currency, string(refund.Status),
		refund.Reason, refund.ProviderRefundID, refund.IdempotencyKey,
		refund.CorrelationID, refund.CausationID,
		refund.CreatedAt, refund.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("refund_repo.Create: %w", err)
	}
	return nil
}

func (r *RefundRepository) FindByID(ctx context.Context, id string) (*domain.Refund, error) {
	query := `SELECT id, payment_id, delivery_id, amount_minor, currency, status, reason,
			  provider_refund_id, idempotency_key, created_at, updated_at, completed_at, failed_at, failure_reason
			  FROM refunds WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	refund, err := scanRefund(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrRefundNotFound
		}
		return nil, fmt.Errorf("refund_repo.FindByID: %w", err)
	}
	return refund, nil
}

func (r *RefundRepository) FindByPaymentID(ctx context.Context, paymentID string) ([]*domain.Refund, error) {
	query := `SELECT id, payment_id, delivery_id, amount_minor, currency, status, reason,
			  provider_refund_id, idempotency_key, created_at, updated_at, completed_at, failed_at, failure_reason
			  FROM refunds WHERE payment_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, paymentID)
	if err != nil {
		return nil, fmt.Errorf("refund_repo.FindByPaymentID: %w", err)
	}
	defer rows.Close()

	var refunds []*domain.Refund
	for rows.Next() {
		refund := &domain.Refund{}
		var status string
		err := rows.Scan(
			&refund.ID, &refund.PaymentID, &refund.DeliveryID,
			&refund.AmountMinor, &refund.Currency, &status, &refund.Reason,
			&refund.ProviderRefundID, &refund.IdempotencyKey,
			&refund.CreatedAt, &refund.UpdatedAt, &refund.CompletedAt, &refund.FailedAt, &refund.FailureReason,
		)
		if err != nil {
			return nil, fmt.Errorf("refund_repo: scan: %w", err)
		}
		refund.Status = domain.RefundStatus(status)
		refunds = append(refunds, refund)
	}
	return refunds, rows.Err()
}

func (r *RefundRepository) UpdateStatus(ctx context.Context, id string, status domain.RefundStatus, providerRefundID string) error {
	query := `UPDATE refunds SET status=$1, provider_refund_id=$2, updated_at=NOW() WHERE id=$3`
	_, err := r.db.ExecContext(ctx, query, string(status), providerRefundID, id)
	if err != nil {
		return fmt.Errorf("refund_repo.UpdateStatus: %w", err)
	}
	return nil
}

func scanRefund(row *sql.Row) (*domain.Refund, error) {
	r := &domain.Refund{}
	var status string
	err := row.Scan(
		&r.ID, &r.PaymentID, &r.DeliveryID,
		&r.AmountMinor, &r.Currency, &status, &r.Reason,
		&r.ProviderRefundID, &r.IdempotencyKey,
		&r.CreatedAt, &r.UpdatedAt, &r.CompletedAt, &r.FailedAt, &r.FailureReason,
	)
	if err != nil {
		return nil, err
	}
	r.Status = domain.RefundStatus(status)
	return r, nil
}
