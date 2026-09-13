-- Migration: 009_create_processed_provider_events.sql
-- Creates the processed_provider_events table for webhook deduplication.

CREATE TABLE processed_provider_events (
    id                  BIGINT PRIMARY KEY,
    provider            TEXT NOT NULL DEFAULT 'stripe',
    provider_event_id   TEXT NOT NULL,
    event_type          TEXT NOT NULL,
    processed_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_processed_provider_events UNIQUE (provider, provider_event_id)
);

CREATE INDEX idx_processed_provider_events_type ON processed_provider_events(event_type);