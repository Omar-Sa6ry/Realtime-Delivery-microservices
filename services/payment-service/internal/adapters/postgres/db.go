package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	_ "github.com/lib/pq"
)

func EnsureDatabaseExists(user, password, host, port, targetDB, sslmode string) error {
	userInfo := url.UserPassword(user, password)
	defaultDSN := fmt.Sprintf("postgres://%s@%s:%s/postgres?sslmode=%s", userInfo.String(), host, port, sslmode)
	
	defaultDB, err := sql.Open("postgres", defaultDSN)
	if err != nil {
		return fmt.Errorf("open default db: %w", err)
	}
	defer defaultDB.Close()

	var lastErr error
	for attempt := 1; attempt <= 60; attempt++ {
		if err := defaultDB.Ping(); err == nil {
			lastErr = nil
			break
		} else {
			lastErr = err
			time.Sleep(2 * time.Second)
		}
	}
	if lastErr != nil {
		return fmt.Errorf("ping default postgres db failed: %w", lastErr)
	}

	var exists bool
	queryCheck := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	if err := defaultDB.QueryRow(queryCheck, targetDB).Scan(&exists); err != nil {
		return fmt.Errorf("check db exists: %w", err)
	}

	if !exists {
		slog.Info("Creating PostgreSQL database...", "database", targetDB)
		queryCreate := fmt.Sprintf("CREATE DATABASE %q", targetDB)
		if _, err := defaultDB.Exec(queryCreate); err != nil {
			return fmt.Errorf("create database %s: %w", targetDB, err)
		}
		slog.Info("Database created successfully", "database", targetDB)
	}

	return nil
}

func Connect(dsn string, user, password, host, port, targetDB, sslmode string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("postgres: DSN is empty — set POSTGRES_DSN")
	}

	// 1. Ensure target database exists
	if user != "" && host != "" && targetDB != "" {
		if err := EnsureDatabaseExists(user, password, host, port, targetDB, sslmode); err != nil {
			slog.Warn("EnsureDatabaseExists non-fatal warning", "error", err)
		}
	}

	// 2. Open connection to target database
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: failed to open connection: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	var lastErr error
	for attempt := 1; attempt <= 60; attempt++ {
		if err := db.Ping(); err == nil {
			slog.Info("PostgreSQL ping succeeded", "attempt", attempt)
			lastErr = nil
			break
		} else {
			lastErr = err
			slog.Warn("PostgreSQL ping failed, retrying in 2s...", "attempt", attempt, "error", err)
			time.Sleep(2 * time.Second)
		}
	}

	if lastErr != nil {
		db.Close()
		return nil, fmt.Errorf("postgres: ping failed after retries: %w", lastErr)
	}

	slog.Info("PostgreSQL connected", "database", targetDB)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := EnsureSchema(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("postgres: failed to ensure schema: %w", err)
	}

	return db, nil
}

func Close(db *sql.DB) {
	if db != nil {
		_ = db.Close()
	}
}

func EnsureSchema(ctx context.Context, db *sql.DB) error {
	schemaDDL := `
	CREATE TABLE IF NOT EXISTS payments (
		id VARCHAR(64) PRIMARY KEY,
		delivery_id VARCHAR(64) NOT NULL,
		user_id VARCHAR(64) NOT NULL,
		amount_minor BIGINT NOT NULL,
		currency VARCHAR(10) NOT NULL,
		status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
		provider VARCHAR(32) NOT NULL DEFAULT 'stripe',
		provider_payment_id VARCHAR(128) NOT NULL DEFAULT '',
		gateway_session_id VARCHAR(256) NOT NULL DEFAULT '',
		authorized_amount_minor BIGINT NOT NULL DEFAULT 0,
		captured_amount_minor BIGINT NOT NULL DEFAULT 0,
		refunded_amount_minor BIGINT NOT NULL DEFAULT 0,
		pending_refund_minor BIGINT NOT NULL DEFAULT 0,
		version BIGINT NOT NULL DEFAULT 0,
		correlation_id VARCHAR(128) NOT NULL DEFAULT '',
		causation_id VARCHAR(128) NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		authorized_at TIMESTAMPTZ,
		captured_at TIMESTAMPTZ,
		cancelled_at TIMESTAMPTZ,
		failed_at TIMESTAMPTZ
	);

	CREATE INDEX IF NOT EXISTS idx_payments_delivery_id ON payments (delivery_id);
	CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments (user_id);
	CREATE INDEX IF NOT EXISTS idx_payments_status ON payments (status);
	CREATE INDEX IF NOT EXISTS idx_payments_provider_payment_id ON payments (provider_payment_id);

	CREATE TABLE IF NOT EXISTS payment_attempts (
		id VARCHAR(64) PRIMARY KEY,
		payment_id VARCHAR(64) NOT NULL,
		attempt_number INT NOT NULL DEFAULT 1,
		operation VARCHAR(32) NOT NULL,
		status VARCHAR(32) NOT NULL DEFAULT 'PROCESSING',
		provider VARCHAR(32) NOT NULL DEFAULT 'stripe',
		provider_idempotency_key VARCHAR(128) NOT NULL DEFAULT '',
		provider_transaction_id VARCHAR(128) NOT NULL DEFAULT '',
		error_code VARCHAR(64) NOT NULL DEFAULT '',
		error_category VARCHAR(64) NOT NULL DEFAULT '',
		safe_error_message TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		completed_at TIMESTAMPTZ
	);

	CREATE INDEX IF NOT EXISTS idx_attempts_payment_id ON payment_attempts (payment_id);
	CREATE INDEX IF NOT EXISTS idx_attempts_status_started ON payment_attempts (status, started_at);

	CREATE TABLE IF NOT EXISTS refunds (
		id VARCHAR(64) PRIMARY KEY,
		payment_id VARCHAR(64) NOT NULL,
		delivery_id VARCHAR(64) NOT NULL,
		amount_minor BIGINT NOT NULL,
		currency VARCHAR(10) NOT NULL,
		status VARCHAR(32) NOT NULL DEFAULT 'REFUND_PENDING',
		reason TEXT NOT NULL DEFAULT '',
		provider_refund_id VARCHAR(128) NOT NULL DEFAULT '',
		idempotency_key VARCHAR(128) NOT NULL DEFAULT '',
		correlation_id VARCHAR(128) NOT NULL DEFAULT '',
		causation_id VARCHAR(128) NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		completed_at TIMESTAMPTZ,
		failed_at TIMESTAMPTZ,
		failure_reason TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_refunds_payment_id ON refunds (payment_id);

	CREATE TABLE IF NOT EXISTS payment_events_outbox (
		id VARCHAR(64) PRIMARY KEY,
		event_type VARCHAR(64) NOT NULL,
		payload BYTEA NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		published_at TIMESTAMPTZ,
		failed_at TIMESTAMPTZ,
		retry_count INT NOT NULL DEFAULT 0,
		error_message TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON payment_events_outbox (published_at) WHERE published_at IS NULL;

	CREATE TABLE IF NOT EXISTS processed_provider_events (
		id BIGSERIAL PRIMARY KEY,
		provider VARCHAR(32) NOT NULL,
		provider_event_id VARCHAR(128) NOT NULL,
		event_type VARCHAR(64) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT uq_processed_provider_event UNIQUE (provider, provider_event_id)
	);

	CREATE TABLE IF NOT EXISTS idempotency_keys (
		key VARCHAR(256) PRIMARY KEY,
		result TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS payment_audit (
		id BIGSERIAL PRIMARY KEY,
		event_type VARCHAR(64) NOT NULL,
		payment_id VARCHAR(64) NOT NULL,
		details JSONB NOT NULL DEFAULT '{}'::jsonb,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_audit_payment_id ON payment_audit (payment_id);
	`

	_, err := db.ExecContext(ctx, schemaDDL)
	if err != nil {
		return fmt.Errorf("EnsureSchema: %w", err)
	}

	slog.Info("PostgreSQL database schema initialized successfully (no external migration files required)")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
