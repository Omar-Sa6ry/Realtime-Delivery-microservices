package nats

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// NestJSRequest represents the JSON packet NestJS expects in NATS transport
type NestJSRequest struct {
	Pattern string      `json:"pattern"`
	Data    interface{} `json:"data"`
	ID      string      `json:"id"`
}

// NestJSResponse represents the JSON packet NestJS responds with
type NestJSResponse struct {
	Response   json.RawMessage `json:"response,omitempty"`
	Error      interface{}     `json:"err,omitempty"`
	ID         string          `json:"id"`
	IsDisposed bool            `json:"isDisposed,omitempty"`
}

// NatsClient wraps the standard NATS connection and adds helper utilities
type NatsClient struct {
	nc *nats.Conn
}

// Connect establishes a new connection to NATS server(s)
func Connect(url string) (*NatsClient, error) {
	if url == "" {
		url = nats.DefaultURL
	}

	opts := []nats.Option{
		nats.Name("Go-Common-Client"),
		nats.Timeout(10 * time.Second),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(5),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			slog.Warn("NATS client disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS client reconnected", "url", nc.ConnectedUrl())
		}),
	}

	nc, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &NatsClient{nc: nc}, nil
}

// Conn returns the raw NATS connection
func (c *NatsClient) Conn() *nats.Conn {
	return c.nc
}

// Close gracefully closes the NATS connection
func (c *NatsClient) Close() {
	if c.nc != nil {
		c.nc.Close()
	}
}

// Publish publishes a standard raw JSON payload to a NATS subject (Pub/Sub)
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

// Request performs a raw JSON Request-Response RPC over NATS
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

// PublishNestJS publishes a payload using the NestJS envelope format (compatible with NestJS event subscribers)
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

// RequestNestJS performs a Request-Response RPC wrapped in the NestJS envelope format (compatible with NestJS @MessagePattern handlers)
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
