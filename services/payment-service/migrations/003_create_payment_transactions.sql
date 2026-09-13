-- Migration: 003_create_payment_transactions.sql
-- Creates the payment_transactions table for tracking individual transactions.

CREATE TABLE payment_transactions (
    id                      BIGINT PRIMARY KEY,
    payment_id              BIGINT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    attempt_id              BIGINT NOT NULL REFERENCES payment_attempts(id) ON DELETE CASCADE,
    transaction_type        TEXT NOT NULL CHECK (transaction_type IN ('AUTHORIZATION', 'CAPTURE', 'VOID', 'REFUND')),
    provider                TEXT NOT NULL DEFAULT 'stripe',
    provider_transaction_id TEXT NOT NULL,
    provider_payment_id     TEXT,
    amount_minor            BIGINT NOT NULL,
    currency                CHAR(3) NOT NULL,
    status                  TEXT NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at            TIMESTAMPTZ
);

CREATE INDEX idx_payment_transactions_payment_id ON payment_transactions(payment_id);
CREATE INDEX idx_payment_transactions_attempt_id ON payment_transactions(attempt_id);
CREATE INDEX idx_payment_transactions_provider_txn ON payment_transactions(provider, provider_transaction_id)
    WHERE provider_transaction_id IS NOT NULL;