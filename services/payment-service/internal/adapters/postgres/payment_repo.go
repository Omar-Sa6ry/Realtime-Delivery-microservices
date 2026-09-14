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

// PaymentRepository implements ports.PaymentRepository using PostgreSQL.
type PaymentRepository struct {
	db *sql.DB
}

// NewPaymentRepository creates a new PaymentRepository.
func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

const paymentCols = `id, delivery_id, user_id, amount_minor, currency, status,
	provider, provider_payment_id, gateway_session_id,
	authorized_amount_minor, captured_amount_minor, refunded_amount_minor, pending_refund_minor,
	version, correlation_id, causation_id,
	created_at, updated_at, authorized_at, captured_at, cancelled_at, failed_at`

// Create inserts a new payment record.
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

// FindByID retrieves a payment by Snowflake ID.
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

// FindByDeliveryID retrieves a payment by delivery ID.
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

// FindByProviderPaymentID retrieves a payment by Stripe PaymentIntent ID.
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

// FindByUserID retrieves all payments for a user (paginated by created_at DESC).
func (r *PaymentRepository) FindByUserID(ctx context.Context, userID string) ([]*domain.Payment, error) {
	query := `SELECT ` + paymentCols + ` FROM payments WHERE user_id = $1 ORDER BY created_at DESC LIMIT 100`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("payment_repo.FindByUserID: %w", err)
	}
	defer rows.Close()
	return scanPayments(rows)
}

// FindByIDs retrieves multiple payments by ID in one query (used by DataLoader).
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

// UpdateConditional updates a payment only if version == expectedVersion (optimistic lock).
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

// scanPayment scans a single row into a Payment.
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

// scanPayments scans multiple rows.
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

// nullTime is a helper for optional time fields.
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
