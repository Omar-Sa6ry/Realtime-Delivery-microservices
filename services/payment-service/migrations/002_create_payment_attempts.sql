-- Migration: 002_create_payment_attempts.sql
-- Creates the payment_attempts table for tracking payment operation attempts.

CREATE TABLE payment_attempts (
    id                      BIGINT PRIMARY KEY,
    payment_id              BIGINT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    attempt_number          INT NOT NULL,
    operation               TEXT NOT NULL CHECK (operation IN ('AUTHORIZE', 'CAPTURE', 'VOID', 'REFUND')),
    status                  TEXT NOT NULL DEFAULT 'NOT_STARTED',
    provider                TEXT NOT NULL DEFAULT 'stripe',
    provider_request_id     TEXT,
    provider_transaction_id TEXT,
    provider_idempotency_key TEXT,
    error_code              TEXT,
    error_category          TEXT,
    safe_error_message      TEXT,
    started_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_attempts_payment_id ON payment_attempts(payment_id);
CREATE INDEX idx_payment_attempts_status ON payment_attempts(status);
CREATE INDEX idx_payment_attempts_provider_request_id ON payment_attempts(provider_request_id)
    WHERE provider_request_id IS NOT NULL;