-- Migration: 001_create_payments.sql
-- Creates the payments table with all necessary fields and constraints.

CREATE TABLE payments (
    id                      BIGINT PRIMARY KEY,
    delivery_id             TEXT NOT NULL UNIQUE,
    user_id                 TEXT NOT NULL,
    currency                CHAR(3) NOT NULL,
    amount_minor            BIGINT NOT NULL CHECK (amount_minor > 0),
    status                  TEXT NOT NULL DEFAULT 'PENDING',
    payment_method_type     TEXT,
    provider                TEXT NOT NULL DEFAULT 'stripe',
    provider_payment_id     TEXT,          -- Stripe PaymentIntent ID
    authorized_amount_minor BIGINT NOT NULL DEFAULT 0,
    captured_amount_minor   BIGINT NOT NULL DEFAULT 0,
    refunded_amount_minor   BIGINT NOT NULL DEFAULT 0,
    pending_refund_minor    BIGINT NOT NULL DEFAULT 0,
    version                 BIGINT NOT NULL DEFAULT 0,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    authorized_at           TIMESTAMPTZ,
    captured_at             TIMESTAMPTZ,
    cancelled_at            TIMESTAMPTZ,
    failed_at               TIMESTAMPTZ,
    CONSTRAINT chk_captured_lte_authorized CHECK (captured_amount_minor <= authorized_amount_minor),
    CONSTRAINT chk_refunded_lte_captured   CHECK (refunded_amount_minor <= captured_amount_minor),
    CONSTRAINT chk_pending_refund_valid    CHECK (
        refunded_amount_minor + pending_refund_minor <= captured_amount_minor
    )
);

CREATE UNIQUE INDEX idx_payments_provider_id ON payments(provider, provider_payment_id)
    WHERE provider_payment_id IS NOT NULL;
CREATE INDEX idx_payments_delivery_id  ON payments(delivery_id);
CREATE INDEX idx_payments_user_created ON payments(user_id, created_at DESC);
CREATE INDEX idx_payments_status       ON payments(status, updated_at);