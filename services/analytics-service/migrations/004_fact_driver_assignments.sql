CREATE TABLE IF NOT EXISTS fact_driver_assignments (
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
ORDER BY (driver_id, offered_at, assignment_id);
