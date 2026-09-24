package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

type Config struct {
	Host        string
	Port        string
	Database    string
	Username    string
	Password    string
	MaxOpenConn int
	MaxIdleConn int
	DialTimeout time.Duration
}

type Client struct {
	db *sql.DB
}

func NewClient(cfg Config) (*Client, error) {
	if cfg.MaxOpenConn <= 0 {
		cfg.MaxOpenConn = 20
	}
	if cfg.MaxIdleConn <= 0 {
		cfg.MaxIdleConn = 5
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 10 * time.Second
	}
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	db := ch.OpenDB(&ch.Options{
		Addr: []string{addr},
		Auth: ch.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialTimeout: cfg.DialTimeout,
	})
	db.SetMaxOpenConns(cfg.MaxOpenConn)
	db.SetMaxIdleConns(cfg.MaxIdleConn)
	db.SetConnMaxLifetime(5 * time.Minute)
	return &Client{db: db}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	var lastErr error
	for attempt := 1; attempt <= 15; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := c.db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return lastErr
}

func (c *Client) DB() *sql.DB {
	return c.db
}

func (c *Client) Close() error {
	return c.db.Close()
}
