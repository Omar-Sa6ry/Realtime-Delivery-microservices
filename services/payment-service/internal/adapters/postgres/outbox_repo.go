package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

type OutboxRepository struct {
	db *sql.DB
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) Insert(ctx context.Context, eventType string, payload []byte) error {
	query := `INSERT INTO payment_events_outbox (event_type, payload, created_at)
			  VALUES ($1, $2, NOW())`
	_, err := r.db.ExecContext(ctx, query, eventType, payload)
	if err != nil {
		return fmt.Errorf("outbox_repo.Insert: %w", err)
	}
	return nil
}

type OutboxRow struct {
	ID        string
	EventType string
	Payload   []byte
	CreatedAt int64
}

func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*OutboxRow, error) {
	query := `SELECT id, event_type, payload, EXTRACT(EPOCH FROM created_at)::BIGINT
			  FROM payment_events_outbox
			  WHERE published_at IS NULL
			  ORDER BY created_at ASC
			  LIMIT $1
			  FOR UPDATE SKIP LOCKED`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("outbox_repo.FetchUnpublished: %w", err)
	}
	defer rows.Close()

	var events []*OutboxRow
	for rows.Next() {
		e := &OutboxRow{}
		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("outbox_repo: scan: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE payment_events_outbox SET published_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("outbox_repo.MarkPublished: %w", err)
	}
	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id string, reason string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE payment_events_outbox SET failed_reason = $1 WHERE id = $2`, reason, id)
	if err != nil {
		return fmt.Errorf("outbox_repo.MarkFailed: %w", err)
	}
	return nil
}

func (r *OutboxRepository) Cleanup(ctx context.Context, olderThanDays int) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM payment_events_outbox
		 WHERE published_at IS NOT NULL
		   AND published_at < NOW() - ($1 || ' days')::INTERVAL`,
		olderThanDays,
	)
	if err != nil {
		return 0, fmt.Errorf("outbox_repo.Cleanup: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

func (r *OutboxRepository) InsertProcessedProviderEvent(ctx context.Context, provider, providerEventID, eventType string) error {
	query := `INSERT INTO processed_provider_events (provider, provider_event_id, event_type, created_at)
			  VALUES ($1, $2, $3, NOW())
			  ON CONFLICT (provider, provider_event_id) DO NOTHING`
	result, err := r.db.ExecContext(ctx, query, provider, providerEventID, eventType)
	if err != nil {
		return fmt.Errorf("outbox_repo.InsertProcessedProviderEvent: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Already processed — this is a duplicate webhook
		return domain.ErrDuplicateIdempotency
	}
	return nil
}
