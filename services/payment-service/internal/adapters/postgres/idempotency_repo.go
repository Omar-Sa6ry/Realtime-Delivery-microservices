package postgres

import (
	"database/sql"
	"fmt"

	"github.com/realtime-delivery/payment-service/internal/domain"
)

type IdempotencyRepository struct {
	db *sql.DB
}

// NewIdempotencyRepository creates a new IdempotencyRepository.
func NewIdempotencyRepository(db *sql.DB) *IdempotencyRepository {
	return &IdempotencyRepository{db: db}
}

// Store saves an idempotency key with its result.
func (r *IdempotencyRepository) Store(key string, result string) error {
	query := `INSERT INTO idempotency_keys (key, result, created_at) VALUES ($1, $2, NOW()) ON CONFLICT (key) DO UPDATE SET result = EXCLUDED.result, created_at = NOW()`
	_, err := r.db.Exec(query, key, result)
	if err != nil {
		return fmt.Errorf("failed to store idempotency key: %w", err)
	}
	return nil
}

// Retrieve retrieves a previously stored result for a key.
func (r *IdempotencyRepository) Retrieve(key string) (string, bool) {
	query := `SELECT result FROM idempotency_keys WHERE key = $1`
	var result string
	err := r.db.QueryRow(query, key).Scan(&result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false
		}
		return "", false
	}
	return result, true
}

// Delete removes a stored idempotency key.
func (r *IdempotencyRepository) Delete(key string) error {
	query := `DELETE FROM idempotency_keys WHERE key = $1`
	result, err := r.db.Exec(query, key)
	if err != nil {
		return fmt.Errorf("failed to delete idempotency key: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("idempotency key not found: %s", key)
	}
	return nil
}

// Exists checks if a key exists.
func (r *IdempotencyRepository) Exists(key string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM idempotency_keys WHERE key = $1)`
	var exists bool
	err := r.db.QueryRow(query, key).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}