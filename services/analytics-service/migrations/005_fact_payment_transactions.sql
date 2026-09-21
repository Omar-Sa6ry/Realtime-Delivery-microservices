CREATE TABLE IF NOT EXISTS fact_payment_transactions (
    event_id            String,
    payment_id          String,
    delivery_id         String,
    user_id             String,
    provider            LowCardinality(String),
    transaction_type    LowCardinality(String),
    status              LowCardinality(String),
    amount              Decimal(18, 2),
    currency            LowCardinality(String),
    provider_latency_ms Nullable(UInt64),
    occurred_at         DateTime64(3, 'UTC'),
    ingested_at         DateTime64(3, 'UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (provider, occurred_at, payment_id);
