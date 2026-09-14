package postgres

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"
)

func Connect(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("postgres: DSN is empty — set POSTGRES_DSN")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: failed to open connection: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("postgres: ping failed: %w", err)
	}

	slog.Info("PostgreSQL connected", "dsn_prefix", dsn[:min(30, len(dsn))])
	return db, nil
}

func Close(db *sql.DB) {
	if db != nil {
		_ = db.Close()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
