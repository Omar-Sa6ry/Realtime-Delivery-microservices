CREATE TABLE IF NOT EXISTS dim_user (
    user_id     String,
    country     LowCardinality(String),
    city        LowCardinality(String),
    signup_date Date,
    user_type   LowCardinality(String),
    created_at  DateTime64(3, 'UTC')
) ENGINE = ReplacingMergeTree(created_at)
ORDER BY (user_id);
