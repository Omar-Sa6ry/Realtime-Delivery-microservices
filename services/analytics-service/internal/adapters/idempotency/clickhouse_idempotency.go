package idempotency

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

var _ ports.IdempotencyStore = (*Store)(nil)

func (s *Store) Exists(ctx context.Context, eventID string) (bool, error) {
	var count uint64
	if err := s.db.QueryRowContext(ctx,
		"SELECT count() FROM analytics_processed_events WHERE event_id = ?", eventID).Scan(&count); err != nil {
		return false, fmt.Errorf("idempotency exists: %w", err)
	}
	return count > 0, nil
}

func (s *Store) Save(ctx context.Context, record *domain.IdempotencyRecord) error {
	if err := record.Validate(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO analytics_processed_events (event_id, aggregate_type, aggregate_id, source_topic, source_offset, first_seen_at) VALUES (?, ?, ?, ?, ?, ?)`,
		record.EventID, record.AggregateType, record.AggregateID, record.SourceTopic, record.SourceOffset, record.FirstSeenAt)
	if err != nil {
		return fmt.Errorf("idempotency save: %w", err)
	}
	return nil
}

func (s *Store) LoadCheckpoint(ctx context.Context, topic string, partition int32) (*domain.IngestionCheckpoint, error) {
	var offset uint64
	var updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT offset, updated_at FROM analytics_checkpoints FINAL WHERE topic = ? AND partition = ?`, topic, partition).Scan(&offset, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load checkpoint: %w", err)
	}
	return &domain.IngestionCheckpoint{
		Topic:     topic,
		Partition: partition,
		Offset:    int64(offset),
		UpdatedAt: updatedAt,
	}, nil
}

func (s *Store) SaveCheckpoint(ctx context.Context, checkpoint *domain.IngestionCheckpoint) error {
	if err := checkpoint.Validate(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO analytics_checkpoints (topic, partition, offset, updated_at) VALUES (?, ?, ?, ?)`,
		checkpoint.Topic, checkpoint.Partition, checkpoint.Offset, checkpoint.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}
	return nil
}

func (s *Store) Close() error { return nil }
