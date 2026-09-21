CREATE MATERIALIZED VIEW IF NOT EXISTS driver_hourly_metrics
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(hour)
ORDER BY (driver_id, hour)
AS SELECT
    toStartOfHour(offered_at) AS hour,
    driver_id,
    count() AS offers,
    countIf(assignment_result = 'ACCEPTED') AS accepted,
    countIf(assignment_result = 'REJECTED') AS rejected,
    countIf(assignment_result = 'EXPIRED') AS expired,
    avg(response_time_ms) AS avg_response_time_ms
FROM fact_driver_assignments
GROUP BY hour, driver_id;
