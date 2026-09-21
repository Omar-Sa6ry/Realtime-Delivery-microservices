CREATE TABLE IF NOT EXISTS fact_notification_events (
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
ORDER BY (channel, occurred_at, notification_id);
