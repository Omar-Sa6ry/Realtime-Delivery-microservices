package postgres

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/realtime-delivery/payment-service/internal/domain"
)

// RefundRepository implements domain.RefundRepo using PostgreSQL.
type RefundRepository struct {
	db *sql.DB
}

// NewRefundRepository creates a new RefundRepository.
func NewRefundRepository(db *sql.DB) *RefundRepository {
	return &RefundRepository{db: db}
}

// Create inserts a new refund into the database.
func (r *RefundRepository) Create(rf *domain.Refund) error {
	query := `INSERT INTO refunds (id, payment_id, amount, currency, reason, status, created_at, completed_at, error)
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.db.Exec(query,
		rf.ID,
		rf.PaymentID,
		rf.Amount,
		rf.Currency,
		rf.Reason,
		string(rf.Status),
		rf.CreatedAt,
		rf.CompletedAt,
		rf.Error,
	)
	if err != nil {
		return fmt.Errorf("failed to create refund: %w", err)
	}
	return nil
}

// Get retrieves a refund by ID.
func (r *RefundRepository) Get(id string) (*domain.Refund, error) {
	query := `SELECT id, payment_id, amount, currency, reason, status, created_at, completed_at, error FROM refunds WHERE id = $1`
	row := r.db.QueryRow(query, id)

	rf := &domain.Refund{}
	err := row.Scan(
		&rf.ID,
		&rf.PaymentID,
		&rf.Amount,
		&rf.Currency,
		&rf.Reason,
		&rf.Status,
		&rf.CreatedAt,
		&rf.CompletedAt,
		&rf.Error,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("refund not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get refund: %w", err)
	}
	return rf, nil
}

// Update updates an existing refund.
func (r *RefundRepository) Update(rf *domain.Refund) error {
	query := `UPDATE refunds SET status = $1, completed_at = $2, error = $3 WHERE id = $4`
	result, err := r.db.Exec(query,
		string(rf.Status),
		rf.CompletedAt,
		rf.Error,
		rf.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update refund: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("refund not found: %s", rf.ID)
	}
	return nil
}

// Delete removes a refund from the database.
func (r *RefundRepository) Delete(id string) error {
	query := `DELETE FROM refunds WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete refund: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("refund not found: %s", id)
	}
	return nil
}

// List retrieves all refunds.
func (r *RefundRepository) List() ([]*domain.Refund, error) {
	query := `SELECT id, payment_id, amount, currency, reason, status, created_at, completed_at, error FROM refunds`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list refunds: %w", err)
	}
	defer rows.Close()

	var refunds []*domain.Refund
	for rows.Next() {
		rf := &domain.Refund{}
		err := rows.Scan(
			&rf.ID,
			&rf.PaymentID,
			&rf.Amount,
			&rf.Currency,
			&rf.Reason,
			&rf.Status,
			&rf.CreatedAt,
			&rf.CompletedAt,
			&rf.Error,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan refund: %w", err)
		}
		refunds = append(refunds, rf)
	}
	return refunds, nil
}

// FindByPaymentID retrieves refunds by payment ID.
func (r *RefundRepository) FindByPaymentID(paymentID string) ([]*domain.Refund, error) {
	query := `SELECT id, payment_id, amount, currency, reason, status, created_at, completed_at, error FROM refunds WHERE payment_id = $1`
	rows, err := r.db.Query(query, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to find refunds by payment ID: %w", err)
	}
	defer rows.Close()

	var refunds []*domain.Refund
	for rows.Next() {
		rf := &domain.Refund{}
		err := rows.Scan(
			&rf.ID,
			&rf.PaymentID,
			&rf.Amount,
			&rf.Currency,
			&rf.Reason,
			&rf.Status,
			&rf.CreatedAt,
			&rf.CompletedAt,
			&rf.Error,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan refund: %w", err)
		}
		refunds = append(refunds, rf)
	}
	return refunds, nil
}