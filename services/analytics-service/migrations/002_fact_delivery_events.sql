CREATE TABLE IF NOT EXISTS fact_delivery_events (
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
ORDER BY (event_type, occurred_at, delivery_id);
