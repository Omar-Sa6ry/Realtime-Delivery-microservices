-- Migration: 004_create_refunds.sql
-- Creates the refunds table.

CREATE TABLE refunds (
    id                      BIGINT PRIMARY KEY,
    payment_id              BIGINT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    delivery_id             TEXT NOT NULL,
    amount_minor            BIGINT NOT NULL CHECK (amount_minor > 0),
    currency                CHAR(3) NOT NULL,
    reason                  TEXT,
    status                  TEXT NOT NULL DEFAULT 'PENDING',
    provider                TEXT NOT NULL DEFAULT 'stripe',
    provider_refund_id      TEXT,
    idempotency_key         TEXT NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at            TIMESTAMPTZ
);

CREATE INDEX idx_refunds_payment_id ON refunds(payment_id);
CREATE INDEX idx_refunds_status ON refunds(status);
CREATE INDEX idx_refunds_delivery_id ON refunds(delivery_id);
CREATE INDEX idx_refunds_idempotency_key ON refunds(idempotency_key) WHERE idempotency_key IS NOT NULL;