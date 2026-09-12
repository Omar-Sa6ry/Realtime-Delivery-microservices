package postgres

import (
	"database/sql"
	"fmt"

	"github.com/realtime-delivery/payment-service/internal/domain"
)

type OutboxRepository struct {
	db *sql.DB
}

// NewOutboxRepository creates a new OutboxRepository.
func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// GetPendingOutboxMessages retrieves pending outbox messages with UPDATE SKIP LOCKED
func (r *OutboxRepository) GetPendingOutboxMessages(limit int) ([]*domain.OutboxMessage, error) {
	query := `SELECT id, event_type, payload, created_at, processed_at, error FROM outbox_messages WHERE status = 'pending' AND processed_at IS NULL LIMIT $1 FOR UPDATE SKIP LOCKED`
	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending outbox messages: %w", err)
	}
	defer rows.Close()

	var messages []*domain.OutboxMessage
	for rows.Next() {
		m := &domain.OutboxMessage{}
		err := rows.Scan(
			&m.ID,
			&m.EventType,
			&m.Payload,
			&m.CreatedAt,
			&m.ProcessedAt,
			&m.Error,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan outbox message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, nil
}

// MarkAsProcessing marks outbox messages as being processed.
func (r *OutboxRepository) MarkAsProcessing(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query := `UPDATE outbox_messages SET status = 'processing', processed_at = NOW() WHERE id = ANY($1)`
	_, err := r.db.Exec(query, ids)
	if err != nil {
		return fmt.Errorf("failed to mark outbox messages as processing: %w", err)
	}
	return nil
}

// MarkAsProcessed marks outbox messages as processed.
func (r *OutboxRepository) MarkAsProcessed(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query := `UPDATE outbox_messages SET status = 'processed', processed_at = NOW(), error = NULL WHERE id = ANY($1)`
	_, err := r.db.Exec(query, ids)
	if err != nil {
		return fmt.Errorf("failed to mark outbox messages as processed: %w", err)
	}
	return nil
}

// MarkAsFailed marks outbox messages as failed.
func (r *OutboxRepository) MarkAsFailed(ids []string, err string) error {
	if len(ids) == 0 {
		return nil
	}
	query := `UPDATE outbox_messages SET status = 'failed', error = $1 WHERE id = ANY($2)`
	_, err2 := r.db.Exec(query, err, ids)
	if err2 != nil {
		return fmt.Errorf("failed to mark outbox messages as failed: %w", err2)
	}
	return nil
}