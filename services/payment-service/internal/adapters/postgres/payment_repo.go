package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
	"github.com/lib/pq"
)

type PaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

const paymentCols = `id, delivery_id, user_id, amount_minor, currency, status,
	provider, provider_payment_id, gateway_session_id,
	authorized_amount_minor, captured_amount_minor, refunded_amount_minor, pending_refund_minor,
	version, correlation_id, causation_id,
	created_at, updated_at, authorized_at, captured_at, cancelled_at, failed_at`

func (r *PaymentRepository) Create(ctx context.Context, p *domain.Payment) error {
	query := `INSERT INTO payments (` + paymentCols + `) VALUES
		($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`

	_, err := r.db.ExecContext(ctx, query,
		p.ID, p.DeliveryID, p.UserID, p.AmountMinor, p.Currency, string(p.Status),
		p.Provider, p.ProviderPaymentID, p.GatewaySessionID,
		p.AuthorizedAmountMinor, p.CapturedAmountMinor, p.RefundedAmountMinor, p.PendingRefundMinor,
		p.Version, p.CorrelationID, p.CausationID,
		p.CreatedAt, p.UpdatedAt, p.AuthorizedAt, p.CapturedAt, p.CancelledAt, p.FailedAt,
	)
	if err != nil {
		return fmt.Errorf("payment_repo.Create: %w", err)
	}
	return nil
}

func (r *PaymentRepository) FindByID(ctx context.Context, id string) (*domain.Payment, error) {
	query := `SELECT ` + paymentCols + ` FROM payments WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	p, err := scanPayment(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("payment_repo.FindByID: %w", err)
	}
	return p, nil
}

func (r *PaymentRepository) FindByDeliveryID(ctx context.Context, deliveryID string) (*domain.Payment, error) {
	query := `SELECT ` + paymentCols + ` FROM payments WHERE delivery_id = $1`
	row := r.db.QueryRowContext(ctx, query, deliveryID)
	p, err := scanPayment(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("payment_repo.FindByDeliveryID: %w", err)
	}
	return p, nil
}

func (r *PaymentRepository) FindByProviderPaymentID(ctx context.Context, providerPaymentID string) (*domain.Payment, error) {
	query := `SELECT ` + paymentCols + ` FROM payments WHERE provider_payment_id = $1`
	row := r.db.QueryRowContext(ctx, query, providerPaymentID)
	p, err := scanPayment(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("payment_repo.FindByProviderPaymentID: %w", err)
	}
	return p, nil
}

func (r *PaymentRepository) FindByUserID(ctx context.Context, userID string) ([]*domain.Payment, error) {
	query := `SELECT ` + paymentCols + ` FROM payments WHERE user_id = $1 ORDER BY created_at DESC LIMIT 100`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("payment_repo.FindByUserID: %w", err)
	}
	defer rows.Close()
	return scanPayments(rows)
}

func (r *PaymentRepository) List(ctx context.Context, page, limit int, userID string) ([]*domain.Payment, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int
	var err error
	var rows *sql.Rows

	if userID != "" {
		countQuery := `SELECT COUNT(*) FROM payments WHERE user_id = $1`
		if err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("payment_repo.List count: %w", err)
		}

		query := `SELECT ` + paymentCols + ` FROM payments WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		rows, err = r.db.QueryContext(ctx, query, userID, limit, offset)
	} else {
		countQuery := `SELECT COUNT(*) FROM payments`
		if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("payment_repo.List count: %w", err)
		}

		query := `SELECT ` + paymentCols + ` FROM payments ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		rows, err = r.db.QueryContext(ctx, query, limit, offset)
	}

	if err != nil {
		return nil, 0, fmt.Errorf("payment_repo.List: %w", err)
	}
	defer rows.Close()

	payments, err := scanPayments(rows)
	if err != nil {
		return nil, 0, err
	}
	return payments, total, nil
}

func (r *PaymentRepository) FindByIDs(ctx context.Context, ids []string) ([]*domain.Payment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := `SELECT ` + paymentCols + ` FROM payments WHERE id = ANY($1)`
	rows, err := r.db.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("payment_repo.FindByIDs: %w", err)
	}
	defer rows.Close()
	return scanPayments(rows)
}

func (r *PaymentRepository) UpdateConditional(ctx context.Context, p *domain.Payment, expectedVersion int64) error {
	query := `UPDATE payments SET
		status=$1, provider_payment_id=$2, gateway_session_id=$3,
		authorized_amount_minor=$4, captured_amount_minor=$5,
		refunded_amount_minor=$6, pending_refund_minor=$7,
		version=$8, updated_at=$9,
		authorized_at=$10, captured_at=$11, cancelled_at=$12, failed_at=$13
	WHERE id=$14 AND version=$15`

	result, err := r.db.ExecContext(ctx, query,
		string(p.Status), p.ProviderPaymentID, p.GatewaySessionID,
		p.AuthorizedAmountMinor, p.CapturedAmountMinor,
		p.RefundedAmountMinor, p.PendingRefundMinor,
		p.Version, p.UpdatedAt,
		p.AuthorizedAt, p.CapturedAt, p.CancelledAt, p.FailedAt,
		p.ID, expectedVersion,
	)
	if err != nil {
		return fmt.Errorf("payment_repo.UpdateConditional: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrConcurrentModification
	}
	return nil
}

func scanPayment(row *sql.Row) (*domain.Payment, error) {
	p := &domain.Payment{}
	var status string
	err := row.Scan(
		&p.ID, &p.DeliveryID, &p.UserID, &p.AmountMinor, &p.Currency, &status,
		&p.Provider, &p.ProviderPaymentID, &p.GatewaySessionID,
		&p.AuthorizedAmountMinor, &p.CapturedAmountMinor, &p.RefundedAmountMinor, &p.PendingRefundMinor,
		&p.Version, &p.CorrelationID, &p.CausationID,
		&p.CreatedAt, &p.UpdatedAt, &p.AuthorizedAt, &p.CapturedAt, &p.CancelledAt, &p.FailedAt,
	)
	if err != nil {
		return nil, err
	}
	p.Status = domain.PaymentStatus(status)
	return p, nil
}

func scanPayments(rows *sql.Rows) ([]*domain.Payment, error) {
	var payments []*domain.Payment
	for rows.Next() {
		p := &domain.Payment{}
		var status string
		err := rows.Scan(
			&p.ID, &p.DeliveryID, &p.UserID, &p.AmountMinor, &p.Currency, &status,
			&p.Provider, &p.ProviderPaymentID, &p.GatewaySessionID,
			&p.AuthorizedAmountMinor, &p.CapturedAmountMinor, &p.RefundedAmountMinor, &p.PendingRefundMinor,
			&p.Version, &p.CorrelationID, &p.CausationID,
			&p.CreatedAt, &p.UpdatedAt, &p.AuthorizedAt, &p.CapturedAt, &p.CancelledAt, &p.FailedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		p.Status = domain.PaymentStatus(status)
		payments = append(payments, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return payments, nil
}

type nullTime struct {
	Time  time.Time
	Valid bool
}

func (n *nullTime) Scan(value interface{}) error {
	if value == nil {
		n.Valid = false
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		n.Time = v
		n.Valid = true
	default:
		return fmt.Errorf("nullTime: unsupported type %T", v)
	}
	return nil
}
