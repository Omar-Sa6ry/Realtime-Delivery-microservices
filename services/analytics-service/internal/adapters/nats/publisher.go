package nats

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	TopicRealtimeMetrics = "analytics.realtime.metrics"
)

type RealtimeMetricPayload struct {
	Timestamp      string  `json:"timestamp"`
	ActiveEvents   int64   `json:"activeEvents"`
	CompletionRate float64 `json:"completionRate"`
	FreshnessSec   float64 `json:"freshnessSec"`
}

type Publisher struct {
	conn *nats.Conn
}

func NewPublisher(url string) (*Publisher, error) {
	nc, err := nats.Connect(url,
		nats.Name("analytics-service"),
		nats.Timeout(5*time.Second),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to nats at %s: %w", url, err)
	}

	slog.Info("connected to nats publisher", "url", url)
	return &Publisher{conn: nc}, nil
}

func (p *Publisher) PublishRealtimeMetrics(payload RealtimeMetricPayload) error {
	if p == nil || p.conn == nil || !p.conn.IsConnected() {
		return nil
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal realtime metrics: %w", err)
	}

	return p.conn.Publish(TopicRealtimeMetrics, data)
}

func (p *Publisher) Close() {
	if p != nil && p.conn != nil {
		p.conn.Drain()
		p.conn.Close()
	}
}
