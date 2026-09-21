package clickhouse

import (
	"context"
	"fmt"
	"log/slog"
)

var ddlStatements = []struct {
	name string
	sql  string
}{
	{
		name: "raw_events",
		sql: `CREATE TABLE IF NOT EXISTS raw_events (
    event_id         String,
    event_type       LowCardinality(String),
    event_version    UInt16,
    aggregate_type   LowCardinality(String),
    aggregate_id     String,
    producer         LowCardinality(String),
    occurred_at      DateTime64(3, 'UTC'),
    ingested_at      DateTime64(3, 'UTC'),
    correlation_id   String,
    causation_id     String,
    source_topic     LowCardinality(String),
    source_partition UInt32,
    source_offset    UInt64,
    payload_json     String
) ENGINE = MergeTree
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (aggregate_type, occurred_at, event_id)
TTL toDateTime(occurred_at) + INTERVAL 90 DAY`,
	},
	{
		name: "fact_delivery_events",
		sql: `CREATE TABLE IF NOT EXISTS fact_delivery_events (
    event_id       String,
    delivery_id    String,
    user_id        String,
    driver_id      String,
    event_type     LowCardinality(String),
    event_version  UInt16,
    city_id        LowCardinality(String),
    zone_id        LowCardinality(String),
    occurred_at    DateTime64(3, 'UTC'),
    ingested_at    DateTime64(3, 'UTC'),
    correlation_id String
) ENGINE = MergeTree
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (event_type, occurred_at, delivery_id)`,
	},
	{
		name: "fact_delivery_completed",
		sql: `CREATE TABLE IF NOT EXISTS fact_delivery_completed (
    delivery_id            String,
    user_id                String,
    driver_id              String,
    created_at             DateTime64(3, 'UTC'),
    assigned_at            Nullable(DateTime64(3, 'UTC')),
    accepted_at            Nullable(DateTime64(3, 'UTC')),
    pickup_started_at      Nullable(DateTime64(3, 'UTC')),
    picked_up_at           Nullable(DateTime64(3, 'UTC')),
    in_transit_at          Nullable(DateTime64(3, 'UTC')),
    delivered_at           Nullable(DateTime64(3, 'UTC')),
    completed_at           Nullable(DateTime64(3, 'UTC')),
    total_duration_seconds Nullable(UInt32),
    assignment_duration_s  Nullable(UInt32),
    pickup_duration_s      Nullable(UInt32),
    transit_duration_s     Nullable(UInt32),
    city_id                LowCardinality(String),
    ingested_at            DateTime64(3, 'UTC')
) ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY toYYYYMM(created_at)
ORDER BY (delivery_id)`,
	},
	{
		name: "fact_driver_assignments",
		sql: `CREATE TABLE IF NOT EXISTS fact_driver_assignments (
    assignment_id     String,
    delivery_id       String,
    driver_id         String,
    offered_at        DateTime64(3, 'UTC'),
    accepted_at       Nullable(DateTime64(3, 'UTC')),
    rejected_at       Nullable(DateTime64(3, 'UTC')),
    expired_at        Nullable(DateTime64(3, 'UTC')),
    released_at       Nullable(DateTime64(3, 'UTC')),
    response_time_ms  Nullable(UInt32),
    assignment_result LowCardinality(String),
    ingested_at       DateTime64(3, 'UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(offered_at)
ORDER BY (driver_id, offered_at, assignment_id)`,
	},
	{
		name: "fact_payment_transactions",
		sql: `CREATE TABLE IF NOT EXISTS fact_payment_transactions (
    event_id            String,
    payment_id          String,
    delivery_id         String,
    user_id             String,
    provider            LowCardinality(String),
    transaction_type    LowCardinality(String),
    status              LowCardinality(String),
    amount              Decimal(18, 2),
    currency            LowCardinality(String),
    provider_latency_ms Nullable(UInt64),
    occurred_at         DateTime64(3, 'UTC'),
    ingested_at         DateTime64(3, 'UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (provider, occurred_at, payment_id)`,
	},
	{
		name: "fact_notification_events",
		sql: `CREATE TABLE IF NOT EXISTS fact_notification_events (
    event_id        String,
    notification_id String,
    recipient_type  LowCardinality(String),
    channel         LowCardinality(String),
    status          LowCardinality(String),
    template_id     LowCardinality(String),
    sent_at         Nullable(DateTime64(3, 'UTC')),
    delivered_at    Nullable(DateTime64(3, 'UTC')),
    failed_at       Nullable(DateTime64(3, 'UTC')),
    retry_count     UInt32,
    error           String,
    occurred_at     DateTime64(3, 'UTC'),
    ingested_at     DateTime64(3, 'UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (channel, occurred_at, notification_id)`,
	},
	{
		name: "analytics_data_quality_issues",
		sql: `CREATE TABLE IF NOT EXISTS analytics_data_quality_issues (
    issue_id       String,
    event_id       String,
    issue_type     LowCardinality(String),
    aggregate_type LowCardinality(String),
    aggregate_id   String,
    detected_at    DateTime64(3, 'UTC'),
    severity       LowCardinality(String),
    details        String,
    resolved_at    Nullable(DateTime64(3, 'UTC'))
) ENGINE = MergeTree
PARTITION BY toYYYYMM(detected_at)
ORDER BY (severity, detected_at, issue_id)`,
	},
}

func Migrate(ctx context.Context, client *Client) error {
	db := client.DB()
	for _, stmt := range ddlStatements {
		if _, err := db.ExecContext(ctx, stmt.sql); err != nil {
			return fmt.Errorf("create table %s: %w", stmt.name, err)
		}
		slog.Info("clickhouse schema ready", "table", stmt.name)
	}
	return nil
}
