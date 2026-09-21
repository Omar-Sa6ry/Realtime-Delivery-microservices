CREATE TABLE IF NOT EXISTS raw_events (
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
TTL occurred_at + INTERVAL 90 DAY;
