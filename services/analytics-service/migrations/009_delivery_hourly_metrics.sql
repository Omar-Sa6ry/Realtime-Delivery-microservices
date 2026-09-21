CREATE MATERIALIZED VIEW IF NOT EXISTS delivery_hourly_metrics
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(hour)
ORDER BY (city_id, hour)
AS SELECT
    toStartOfHour(occurred_at) AS hour,
    city_id,
    countIf(event_type = 'delivery.created') AS total_deliveries,
    countIf(event_type = 'delivery.completed') AS completed_deliveries,
    countIf(event_type = 'delivery.cancelled') AS cancelled_deliveries,
    countIf(event_type = 'delivery.failed') AS failed_deliveries
FROM fact_delivery_events
GROUP BY hour, city_id;
