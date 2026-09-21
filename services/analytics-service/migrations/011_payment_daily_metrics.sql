CREATE MATERIALIZED VIEW IF NOT EXISTS payment_daily_metrics
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(day)
ORDER BY (provider, day)
AS SELECT
    toStartOfDay(occurred_at) AS day,
    provider,
    countIf(transaction_type = 'AUTHORIZATION') AS authorization_count,
    countIf(transaction_type = 'AUTHORIZATION' AND status = 'SUCCEEDED') AS authorization_success_count,
    countIf(transaction_type = 'CAPTURE') AS capture_count,
    sumIf(amount, transaction_type = 'CAPTURE') AS captured_amount,
    countIf(transaction_type = 'REFUND') AS refund_count,
    sumIf(amount, transaction_type = 'REFUND') AS refunded_amount,
    avg(provider_latency_ms) AS avg_provider_latency_ms
FROM fact_payment_transactions
GROUP BY day, provider;
