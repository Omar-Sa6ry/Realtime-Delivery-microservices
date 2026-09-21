CREATE TABLE IF NOT EXISTS analytics_data_quality_issues (
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
ORDER BY (severity, detected_at, issue_id);
