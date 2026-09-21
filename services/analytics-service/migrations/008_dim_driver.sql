CREATE TABLE IF NOT EXISTS dim_driver (
    driver_id   String,
    vehicle_type LowCardinality(String),
    city        LowCardinality(String),
    created_at  DateTime64(3, 'UTC')
) ENGINE = ReplacingMergeTree(created_at)
ORDER BY (driver_id);
