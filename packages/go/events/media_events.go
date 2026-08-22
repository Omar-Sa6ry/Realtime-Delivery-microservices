package events

import (
	"encoding/json"
	"fmt"
	"time"
)

// MediaEventType is the canonical event type identifier for Kafka messages.
type MediaEventType string

const (
	MediaUploadCreated       MediaEventType = "media.upload.created"
	MediaUploadCompleted     MediaEventType = "media.upload.completed"
	MediaUploadAborted       MediaEventType = "media.upload.aborted"
	MediaScanStarted         MediaEventType = "media.scan.started"
	MediaScanCompleted       MediaEventType = "media.scan.completed"
	MediaScanFailed          MediaEventType = "media.scan.failed"
	MediaProcessingStarted   MediaEventType = "media.processing.started"
	MediaProcessingCompleted MediaEventType = "media.processing.completed"
	MediaProcessingFailed    MediaEventType = "media.processing.failed"
	MediaReady               MediaEventType = "media.ready"
	MediaDeleteRequested     MediaEventType = "media.delete.requested"
	MediaDeleted             MediaEventType = "media.deleted"
	MediaDeleteFailed        MediaEventType = "media.delete.failed"
)

// MediaUploadCreatedPayload is emitted when a new upload session is created.
type MediaUploadCreatedPayload struct {
	MediaID     string `json:"mediaId"`
	UserID      string `json:"userId"`
	DeliveryID  string `json:"deliveryId,omitempty"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	MediaType   string `json:"mediaType"`
	Size        int64  `json:"size"`
	UploadID    string `json:"uploadId"`
}

// MediaUploadCompletedPayload is emitted when a multipart upload is finalised.
type MediaUploadCompletedPayload struct {
	MediaID     string `json:"mediaId"`
	UserID      string `json:"userId"`
	ObjectKey   string `json:"objectKey"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	MediaType   string `json:"mediaType"`
}

// MediaReadyPayload is emitted when all processing is complete and the media is available.
type MediaReadyPayload struct {
	MediaID     string             `json:"mediaId"`
	UserID      string             `json:"userId"`
	MediaType   string             `json:"mediaType"`
	ContentType string             `json:"contentType"`
	Versions    []MediaVersionInfo `json:"versions"`
}

// MediaVersionInfo describes one processed rendition.
type MediaVersionInfo struct {
	VersionType string `json:"versionType"`
	ObjectKey   string `json:"objectKey"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Width       int32  `json:"width,omitempty"`
	Height      int32  `json:"height,omitempty"`
	DurationMS  int64  `json:"durationMs,omitempty"`
}

// MediaDeletedPayload is emitted when a media item is fully deleted from S3.
type MediaDeletedPayload struct {
	MediaID string    `json:"mediaId"`
	UserID  string    `json:"userId"`
	At      time.Time `json:"at"`
}

// MediaScanCompletedPayload is emitted after antivirus scanning.
type MediaScanCompletedPayload struct {
	MediaID  string `json:"mediaId"`
	UserID   string `json:"userId"`
	Infected bool   `json:"infected"`
	Threat   string `json:"threat,omitempty"`
}

type EventEnvelope struct {
	EventID   string          `json:"eventId"`
	EventType string          `json:"eventType"` // e.g. "delivery.created", "media.ready"
	TraceID   string          `json:"traceId,omitempty"`
	Timestamp int64           `json:"timestamp"` // unix milliseconds
	Payload   json.RawMessage `json:"payload"`
}

func NewEventEnvelope(eventID string, eventType string, traceID string, payload interface{}) (*EventEnvelope, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}
	return &EventEnvelope{
		EventID:   eventID,
		EventType: eventType,
		TraceID:   traceID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payloadBytes,
	}, nil
}

func NewMediaEventEnvelope(eventID string, eventType MediaEventType, traceID string, payload interface{}) (*EventEnvelope, error) {
	return NewEventEnvelope(eventID, string(eventType), traceID, payload)
}

// Accepts any string event type for cross-domain use.
func MarshalEnvelope(eventID string, eventType string, traceID string, payload interface{}) ([]byte, error) {
	env, err := NewEventEnvelope(eventID, eventType, traceID, payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(env)
}

// MarshalMediaEnvelope is a backward-compatible wrapper for media events.
func MarshalMediaEnvelope(eventID string, eventType MediaEventType, traceID string, payload interface{}) ([]byte, error) {
	return MarshalEnvelope(eventID, string(eventType), traceID, payload)
}

// UnmarshalEnvelope parses a raw Kafka/NATS message into an EventEnvelope.
func UnmarshalEnvelope(data []byte) (*EventEnvelope, error) {
	var env EventEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("unmarshal event envelope: %w", err)
	}
	return &env, nil
}
