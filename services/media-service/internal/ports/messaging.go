package ports

import "context"

// EventPublisher abstracts event publishing to Kafka.
// The Outbox pattern ensures events are never lost:
// events are first persisted in DynamoDB, then published here by the outbox worker.
type EventPublisher interface {
	// Publish sends a single event to the appropriate Kafka topic.
	// The topic is derived from the event type.
	Publish(ctx context.Context, topic string, key string, payload []byte, traceID string) error

	// Close gracefully shuts down the publisher, flushing any pending messages.
	Close() error
}

// VirusScanner abstracts malware scanning for uploaded objects.
// The default implementation delegates to ClamAV via TCP.
type VirusScanner interface {
	// ScanObject downloads the object from S3 and scans it for malware.
	// Returns infected=true and a threat name if a threat is found.
	// Returns an error for transient failures (e.g. ClamAV unavailable) — these are retried.
	ScanObject(ctx context.Context, objectKey string) (infected bool, threat string, err error)
}
