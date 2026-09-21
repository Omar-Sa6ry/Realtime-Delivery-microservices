CREATE TABLE IF NOT EXISTS analytics_checkpoints (
    topic      LowCardinality(String),
    partition  UInt32,
    offset     UInt64,
    updated_at DateTime64(3, 'UTC')
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (topic, partition);
