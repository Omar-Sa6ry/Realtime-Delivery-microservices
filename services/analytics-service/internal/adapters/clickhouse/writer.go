package clickhouse

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

type Writer struct {
	db *sql.DB
}

func NewWriter(client *Client) *Writer {
	return &Writer{db: client.DB()}
}

var _ ports.ClickHouseWriter = (*Writer)(nil)

func execBatch(ctx context.Context, db *sql.DB, query string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()
	for i, args := range rows {
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("exec row %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (w *Writer) WriteRawEvents(ctx context.Context, rows []*domain.RawEventLanding) error {
	const q = `INSERT INTO raw_events (event_id, event_type, event_version, aggregate_type, aggregate_id, producer, occurred_at, ingested_at, correlation_id, causation_id, source_topic, source_partition, source_offset, payload_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	batch := make([][]any, 0, len(rows))
	for _, r := range rows {
		batch = append(batch, []any{r.EventID, r.EventType, uint16(r.EventVersion), r.AggregateType, r.AggregateID, r.Producer, r.OccurredAt, r.IngestedAt, r.CorrelationID, r.CausationID, r.SourceTopic, r.SourcePartition, r.SourceOffset, r.PayloadJSON})
	}
	return execBatch(ctx, w.db, q, batch)
}

func (w *Writer) WriteDeliveryEvents(ctx context.Context, rows []*domain.FactDeliveryEvent) error {
	const q = `INSERT INTO fact_delivery_events (event_id, delivery_id, user_id, driver_id, event_type, event_version, city_id, zone_id, occurred_at, ingested_at, correlation_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	batch := make([][]any, 0, len(rows))
	for _, r := range rows {
		batch = append(batch, []any{r.EventID, r.DeliveryID, r.UserID, r.DriverID, r.EventType, uint16(r.EventVersion), r.CityID, r.ZoneID, r.OccurredAt, r.IngestedAt, r.CorrelationID})
	}
	return execBatch(ctx, w.db, q, batch)
}

func (w *Writer) WriteDeliveryCompleted(ctx context.Context, rows []*domain.FactDeliveryCompleted) error {
	const q = `INSERT INTO fact_delivery_completed (delivery_id, user_id, driver_id, created_at, assigned_at, accepted_at, pickup_started_at, picked_up_at, in_transit_at, delivered_at, completed_at, total_duration_seconds, assignment_duration_s, pickup_duration_s, transit_duration_s, city_id, ingested_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	batch := make([][]any, 0, len(rows))
	for _, r := range rows {
		batch = append(batch, []any{r.DeliveryID, r.UserID, r.DriverID, r.CreatedAt, r.AssignedAt, r.AcceptedAt, r.PickupStartedAt, r.PickedUpAt, r.InTransitAt, r.DeliveredAt, r.CompletedAt, r.TotalDurationS, r.AssignmentDurationS, r.PickupDurationS, r.TransitDurationS, r.CityID, r.IngestedAt})
	}
	return execBatch(ctx, w.db, q, batch)
}

func (w *Writer) WriteDriverAssignments(ctx context.Context, rows []*domain.FactDriverAssignment) error {
	const q = `INSERT INTO fact_driver_assignments (assignment_id, delivery_id, driver_id, offered_at, accepted_at, rejected_at, expired_at, released_at, response_time_ms, assignment_result, ingested_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	batch := make([][]any, 0, len(rows))
	for _, r := range rows {
		batch = append(batch, []any{r.AssignmentID, r.DeliveryID, r.DriverID, r.OfferedAt, r.AcceptedAt, r.RejectedAt, r.ExpiredAt, r.ReleasedAt, r.ResponseTimeMs, string(r.Result), r.IngestedAt})
	}
	return execBatch(ctx, w.db, q, batch)
}

func (w *Writer) WritePaymentTransactions(ctx context.Context, rows []*domain.FactPaymentTransaction) error {
	const q = `INSERT INTO fact_payment_transactions (event_id, payment_id, delivery_id, user_id, provider, transaction_type, status, amount, currency, provider_latency_ms, occurred_at, ingested_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	batch := make([][]any, 0, len(rows))
	for _, r := range rows {
		amount, err := decimal.NewFromString(r.Amount)
		if err != nil {
			return fmt.Errorf("%w: %q", domain.ErrInvalidAmountFormat, r.Amount)
		}
		batch = append(batch, []any{r.EventID, r.PaymentID, r.DeliveryID, r.UserID, r.Provider, string(r.TransactionType), r.Status, amount, r.Currency, r.ProviderLatencyMs, r.OccurredAt, r.IngestedAt})
	}
	return execBatch(ctx, w.db, q, batch)
}

func (w *Writer) WriteNotificationEvents(ctx context.Context, rows []*domain.FactNotificationEvent) error {
	const q = `INSERT INTO fact_notification_events (event_id, notification_id, recipient_type, channel, status, template_id, sent_at, delivered_at, failed_at, retry_count, error, occurred_at, ingested_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	batch := make([][]any, 0, len(rows))
	for _, r := range rows {
		batch = append(batch, []any{r.EventID, r.NotificationID, r.RecipientType, string(r.Channel), string(r.Status), r.TemplateID, r.SentAt, r.DeliveredAt, r.FailedAt, uint32(r.RetryCount), r.Error, r.OccurredAt, r.IngestedAt})
	}
	return execBatch(ctx, w.db, q, batch)
}

func (w *Writer) WriteDataQualityIssues(ctx context.Context, rows []*domain.DataQualityIssue) error {
	const q = `INSERT INTO analytics_data_quality_issues (issue_id, event_id, issue_type, aggregate_type, aggregate_id, detected_at, severity, details, resolved_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	batch := make([][]any, 0, len(rows))
	for _, r := range rows {
		batch = append(batch, []any{r.IssueID, r.EventID, r.IssueType, r.AggregateType, r.AggregateID, r.DetectedAt, string(r.Severity), r.Details, r.ResolvedAt})
	}
	return execBatch(ctx, w.db, q, batch)
}

func (w *Writer) Close() error { return nil }
