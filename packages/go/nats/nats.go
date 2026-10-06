package nats

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type NestJSRequest struct {
	Pattern string      `json:"pattern"`
	Data    interface{} `json:"data"`
	ID      string      `json:"id"`
}

type NestJSResponse struct {
	Response   json.RawMessage `json:"response,omitempty"`
	Error      interface{}     `json:"err,omitempty"`
	ID         string          `json:"id"`
	IsDisposed bool            `json:"isDisposed,omitempty"`
}

type Conn interface {
	Publish(subject string, data []byte) error
	Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error)
	Close()
}

type NatsClient struct {
	nc  Conn
	raw *nats.Conn
}

// Swappable dial/retry knobs so tests never block on a real broker.
var (
	natsDial         = nats.Connect
	connectAttempts  = 15
	connectRetryWait = 2 * time.Second
)

func Connect(url string) (*NatsClient, error) {
	if url == "" {
		url = nats.DefaultURL
	}

	opts := []nats.Option{
		nats.Name("Go-Common-Client"),
		nats.Timeout(10 * time.Second),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			slog.Warn("NATS client disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS client reconnected", "url", nc.ConnectedUrl())
		}),
	}

	maxAttempts := connectAttempts
	retryDelay := connectRetryWait

	var nc *nats.Conn
	var lastErr error
	for i := 1; i <= maxAttempts; i++ {
		nc, lastErr = natsDial(url, opts...)
		if lastErr == nil {
			slog.Info("NATS client connected successfully", "url", url, "attempt", i)
			return &NatsClient{nc: nc, raw: nc}, nil
		}
		slog.Warn("NATS client connection failed, retrying...", "url", url, "attempt", i, "maxAttempts", maxAttempts, "error", lastErr)
		if i < maxAttempts {
			time.Sleep(retryDelay)
		}
	}

	return nil, fmt.Errorf("failed to connect to NATS after %d attempts: %w", maxAttempts, lastErr)
}

// newClient wraps an already-connected NATS surface (used by tests).
func newClient(nc Conn) *NatsClient {
	return &NatsClient{nc: nc}
}

func (c *NatsClient) Conn() *nats.Conn {
	return c.raw
}

func (c *NatsClient) Close() {
	if c.nc != nil {
		c.nc.Close()
	}
}

func (c *NatsClient) Publish(subject string, data interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal publish payload: %w", err)
	}

	err = c.nc.Publish(subject, bytes)
	if err != nil {
		slog.Error("NATS publish failed", "subject", subject, "error", err)
		return err
	}

	slog.Debug("Event published raw", "subject", subject)
	return nil
}

func (c *NatsClient) Request(subject string, data interface{}, timeout time.Duration) ([]byte, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	msg, err := c.nc.Request(subject, bytes, timeout)
	if err != nil {
		slog.Error("NATS request failed", "subject", subject, "error", err)
		return nil, err
	}

	return msg.Data, nil
}

func (c *NatsClient) PublishNestJS(pattern string, data interface{}) error {
	envelope := NestJSRequest{
		Pattern: pattern,
		Data:    data,
		ID:      uuid.NewString(),
	}

	bytes, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal NestJS publish envelope: %w", err)
	}

	err = c.nc.Publish(pattern, bytes)
	if err != nil {
		slog.Error("NATS NestJS publish failed", "pattern", pattern, "error", err)
		return err
	}

	slog.Debug("Event published NestJS", "pattern", pattern)
	return nil
}

func (c *NatsClient) RequestNestJS(pattern string, data interface{}, timeout time.Duration) (json.RawMessage, error) {
	envelope := NestJSRequest{
		Pattern: pattern,
		Data:    data,
		ID:      uuid.NewString(),
	}

	bytes, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal NestJS request envelope: %w", err)
	}

	msg, err := c.nc.Request(pattern, bytes, timeout)
	if err != nil {
		slog.Error("NATS NestJS request failed", "pattern", pattern, "error", err)
		return nil, err
	}

	var res NestJSResponse
	if err := json.Unmarshal(msg.Data, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal NestJS response envelope: %w", err)
	}

	if res.Error != nil {
		return nil, fmt.Errorf("NestJS RPC error: %v", res.Error)
	}

	return res.Response, nil
}
