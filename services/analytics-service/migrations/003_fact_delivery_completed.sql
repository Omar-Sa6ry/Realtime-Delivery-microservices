CREATE TABLE IF NOT EXISTS fact_delivery_completed (
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
ORDER BY (delivery_id);
