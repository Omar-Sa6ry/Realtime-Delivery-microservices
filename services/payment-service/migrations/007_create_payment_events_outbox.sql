-- Migration: 007_create_payment_events_outbox.sql
-- Creates the payment_events_outbox table for the outbox pattern.

CREATE TABLE payment_events_outbox (
    id              BIGINT PRIMARY KEY,
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    error           TEXT,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at    TIMESTAMPTZ,
    retries         INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_outbox_status_published ON payment_events_outbox(status, occurred_at)
    WHERE status = 'pending' AND published_at IS NULL;
CREATE INDEX idx_outbox_published ON payment_events_outbox(published_at);