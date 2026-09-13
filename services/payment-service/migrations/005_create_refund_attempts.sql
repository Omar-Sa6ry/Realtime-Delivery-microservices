-- Migration: 005_create_refund_attempts.sql
-- Creates the refund_attempts table for tracking refund execution attempts.

CREATE TABLE refund_attempts (
    id                      BIGINT PRIMARY KEY,
    refund_id               BIGINT NOT NULL REFERENCES refunds(id) ON DELETE CASCADE,
    attempt_number          INT NOT NULL,
    status                  TEXT NOT NULL DEFAULT 'PROCESSING',
    provider                TEXT NOT NULL DEFAULT 'stripe',
    provider_refund_id      TEXT,
    provider_idempotency_key TEXT,
    error_code              TEXT,
    error_category          TEXT,
    safe_error_message      TEXT,
    started_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refund_attempts_refund_id ON refund_attempts(refund_id);
CREATE INDEX idx_refund_attempts_status ON refund_attempts(status);