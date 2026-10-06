package events

import "time"

type SearchEventType string

const (
	// NATS transient signals — low-latency, no durability required.
	SearchQueryStarted   SearchEventType = "search.query.started"
	SearchQueryCompleted SearchEventType = "search.query.completed"

	// Kafka events — durable, consumed by Analytics Service / ClickHouse.
	SearchReindexStarted   SearchEventType = "search.reindex.started"
	SearchReindexCompleted SearchEventType = "search.reindex.completed"
	SearchReindexFailed    SearchEventType = "search.reindex.failed"
)

type SearchQueryStartedPayload struct {
	QueryHash string    `json:"queryHash"` // SHA256 of canonical query
	Index     string    `json:"index"`     // deliveries | drivers | media
	UserID    string    `json:"userId,omitempty"`
	TraceID   string    `json:"traceId,omitempty"`
	StartedAt time.Time `json:"startedAt"`
}

type SearchQueryCompletedPayload struct {
	QueryHash    string        `json:"queryHash"`
	Index        string        `json:"index"`
	UserID       string        `json:"userId,omitempty"`
	TraceID      string        `json:"traceId,omitempty"`
	LatencyMs    int64         `json:"latencyMs"`
	ResultCount  int           `json:"resultCount"`
	CacheHit     bool          `json:"cacheHit"`
	ZeroResults  bool          `json:"zeroResults"`
	CompletedAt  time.Time     `json:"completedAt"`
}

type SearchReindexStartedPayload struct {
	JobID     string    `json:"jobId"`
	Index     string    `json:"index"`
	StartedAt time.Time `json:"startedAt"`
	TriggeredBy string  `json:"triggeredBy"` // admin | scheduled | reconciliation
}

type SearchReindexCompletedPayload struct {
	JobID          string        `json:"jobId"`
	Index          string        `json:"index"`
	DocumentsTotal int64         `json:"documentsTotal"`
	DocumentsFailed int64        `json:"documentsFailed"`
	DurationMs     int64         `json:"durationMs"`
	CompletedAt    time.Time     `json:"completedAt"`
}

type SearchReindexFailedPayload struct {
	JobID     string    `json:"jobId"`
	Index     string    `json:"index"`
	Error     string    `json:"error"`
	FailedAt  time.Time `json:"failedAt"`
}
