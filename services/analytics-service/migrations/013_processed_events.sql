CREATE TABLE IF NOT EXISTS analytics_processed_events (
    event_id       String,
    aggregate_type LowCardinality(String),
    aggregate_id   String,
    source_topic   LowCardinality(String),
    source_offset  UInt64,
    first_seen_at  DateTime64(3, 'UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(first_seen_at)
ORDER BY (event_id)
TTL first_seen_at + INTERVAL 30 DAY;
