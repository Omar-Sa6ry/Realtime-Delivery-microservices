-- Migration: 006_create_idempotency_keys.sql
-- Creates the idempotency_keys table for deduplication.

CREATE TABLE idempotency_keys (
    key                     TEXT PRIMARY KEY,
    operation               TEXT NOT NULL,
    request_hash            TEXT NOT NULL,
    response_payload        JSONB,
    status                  TEXT NOT NULL DEFAULT 'pending',
    expires_at              TIMESTAMPTZ NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at            TIMESTAMPTZ
);

CREATE INDEX idx_idempotency_keys_expires_at ON idempotency_keys(expires_at)
    WHERE status = 'pending';