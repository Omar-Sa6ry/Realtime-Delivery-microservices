-- Migration: 008_create_payment_audit_log.sql
-- Creates the payment_audit_log table for audit trail.

CREATE TABLE payment_audit_log (
    id              BIGINT PRIMARY KEY,
    payment_id      BIGINT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    operation       TEXT NOT NULL,
    actor           TEXT, -- user_id, system, or provider
    details         JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_log_payment_id ON payment_audit_log(payment_id, created_at DESC);