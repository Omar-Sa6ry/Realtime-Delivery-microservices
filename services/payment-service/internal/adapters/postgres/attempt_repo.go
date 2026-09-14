package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

type AttemptRepository struct {
	db *sql.DB
}

func NewAttemptRepository(db *sql.DB) *AttemptRepository {
	return &AttemptRepository{db: db}
}

func (r *AttemptRepository) Create(ctx context.Context, a *domain.Attempt) error {
	query := `INSERT INTO payment_attempts
		(id, payment_id, attempt_number, operation, status, provider,
		 provider_idempotency_key, created_at, started_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := r.db.ExecContext(ctx, query,
		a.ID, a.PaymentID, a.AttemptNumber, a.Operation,
		string(a.Status), a.Provider, a.ProviderIdempotencyKey,
		a.CreatedAt, a.StartedAt,
	)
	if err != nil {
		return fmt.Errorf("attempt_repo.Create: %w", err)
	}
	return nil
}

func (r *AttemptRepository) FindByPaymentID(ctx context.Context, paymentID string) ([]*domain.Attempt, error) {
	query := `SELECT id, payment_id, attempt_number, operation, status, provider,
			  provider_idempotency_key, provider_transaction_id, error_code, error_category,
			  safe_error_message, created_at, started_at, completed_at
			  FROM payment_attempts WHERE payment_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, query, paymentID)
	if err != nil {
		return nil, fmt.Errorf("attempt_repo.FindByPaymentID: %w", err)
	}
	defer rows.Close()

	var attempts []*domain.Attempt
	for rows.Next() {
		a := &domain.Attempt{}
		var status string
		err := rows.Scan(
			&a.ID, &a.PaymentID, &a.AttemptNumber, &a.Operation, &status, &a.Provider,
			&a.ProviderIdempotencyKey, &a.ProviderTransactionID,
			&a.ErrorCode, &a.ErrorCategory, &a.SafeErrorMessage,
			&a.CreatedAt, &a.StartedAt, &a.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("attempt_repo: scan: %w", err)
		}
		a.Status = domain.OperationStatus(status)
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}

func (r *AttemptRepository) UpdateStatus(ctx context.Context, id string, status domain.OperationStatus, providerTxID string) error {
	query := `UPDATE payment_attempts SET status=$1, provider_transaction_id=$2, completed_at=NOW() WHERE id=$3`
	_, err := r.db.ExecContext(ctx, query, string(status), providerTxID, id)
	if err != nil {
		return fmt.Errorf("attempt_repo.UpdateStatus: %w", err)
	}
	return nil
}

func (r *AttemptRepository) FindStuck(ctx context.Context, thresholdSeconds int) ([]*domain.Attempt, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(thresholdSeconds) * time.Second)
	query := `SELECT id, payment_id, operation, provider, provider_idempotency_key, provider_transaction_id
			  FROM payment_attempts
			  WHERE status = 'PROCESSING'
			    AND started_at < $1
			  LIMIT 50`
	rows, err := r.db.QueryContext(ctx, query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("attempt_repo.FindStuck: %w", err)
	}
	defer rows.Close()

	var attempts []*domain.Attempt
	for rows.Next() {
		a := &domain.Attempt{Status: domain.OperationStatusProcessing}
		err := rows.Scan(&a.ID, &a.PaymentID, &a.Operation, &a.Provider, &a.ProviderIdempotencyKey, &a.ProviderTransactionID)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, nil
}

func (r *AttemptRepository) FindUnknown(ctx context.Context, limit int) ([]*domain.Attempt, error) {
	query := `SELECT id, payment_id, operation, provider, provider_idempotency_key, provider_transaction_id
			  FROM payment_attempts
			  WHERE status = 'UNKNOWN'
			  LIMIT $1`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("attempt_repo.FindUnknown: %w", err)
	}
	defer rows.Close()

	var attempts []*domain.Attempt
	for rows.Next() {
		a := &domain.Attempt{Status: domain.OperationStatusUnknown}
		err := rows.Scan(&a.ID, &a.PaymentID, &a.Operation, &a.Provider, &a.ProviderIdempotencyKey, &a.ProviderTransactionID)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, nil
}
