package postgres

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/realtime-delivery/payment-service/internal/domain"
)

// AttemptRepository implements domain.AttemptRepo using PostgreSQL.
type AttemptRepository struct {
	db *sql.DB
}

// NewAttemptRepository creates a new AttemptRepository.
func NewAttemptRepository(db *sql.DB) *AttemptRepository {
	return &AttemptRepository{db: db}
}

// Create inserts a new attempt into the database.
func (r *AttemptRepository) Create(a *domain.Attempt) error {
	query := `INSERT INTO attempts (id, payment_id, amount, currency, status, created_at, completed_at, error)
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.db.Exec(query,
		a.ID,
		a.PaymentID,
		a.Amount,
		a.Currency,
		a.Status,
		a.CreatedAt,
		a.CompletedAt,
		a.Error,
	)
	if err != nil {
		return fmt.Errorf("failed to create attempt: %w", err)
	}
	return nil
}

// Get retrieves an attempt by ID.
func (r *AttemptRepository) Get(id string) (*domain.Attempt, error) {
	query := `SELECT id, payment_id, amount, currency, status, created_at, completed_at, error FROM attempts WHERE id = $1`
	row := r.db.QueryRow(query, id)

	a := &domain.Attempt{}
	err := row.Scan(
		&a.ID,
		&a.PaymentID,
		&a.Amount,
		&a.Currency,
		&a.Status,
		&a.CreatedAt,
		&a.CompletedAt,
		&a.Error,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("attempt not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get attempt: %w", err)
	}
	return a, nil
}

// Update updates an existing attempt.
func (r *AttemptRepository) Update(a *domain.Attempt) error {
	query := `UPDATE attempts SET status = $1, completed_at = $2, error = $3 WHERE id = $4`
	result, err := r.db.Exec(query,
		a.Status,
		a.CompletedAt,
		a.Error,
		a.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update attempt: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("attempt not found: %s", a.ID)
	}
	return nil
}

// Delete removes an attempt from the database.
func (r *AttemptRepository) Delete(id string) error {
	query := `DELETE FROM attempts WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete attempt: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("attempt not found: %s", id)
	}
	return nil
}

// List retrieves all attempts.
func (r *AttemptRepository) List() ([]*domain.Attempt, error) {
	query := `SELECT id, payment_id, amount, currency, status, created_at, completed_at, error FROM attempts`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list attempts: %w", err)
	}
	defer rows.Close()

	var attempts []*domain.Attempt
	for rows.Next() {
		a := &domain.Attempt{}
		err := rows.Scan(
			&a.ID,
			&a.PaymentID,
			&a.Amount,
			&a.Currency,
			&a.Status,
			&a.CreatedAt,
			&a.CompletedAt,
			&a.Error,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attempt: %w", err)
		}
		attempts = append(attempts, a)
	}
	return attempts, nil
}

// FindByPaymentID retrieves attempts by payment ID.
func (r *AttemptRepository) FindByPaymentID(paymentID string) ([]*domain.Attempt, error) {
	query := `SELECT id, payment_id, amount, currency, status, created_at, completed_at, error FROM attempts WHERE payment_id = $1`
	rows, err := r.db.Query(query, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to find attempts by payment ID: %w", err)
	}
	defer rows.Close()

	var attempts []*domain.Attempt
	for rows.Next() {
		a := &domain.Attempt{}
		err := rows.Scan(
			&a.ID,
			&a.PaymentID,
			&a.Amount,
			&a.Currency,
			&a.Status,
			&a.CreatedAt,
			&a.CompletedAt,
			&a.Error,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attempt: %w", err)
		}
		attempts = append(attempts, a)
	}
	return attempts, nil
}