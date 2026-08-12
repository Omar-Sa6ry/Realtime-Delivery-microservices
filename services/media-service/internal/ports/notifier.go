package ports

import (
	"context"
	"time"
)

// MediaEventType represents the type of real-time notification pushed to clients.
type MediaEventType string

const (
	// Upload lifecycle events
	EventUploadCreated   MediaEventType = "UPLOAD_CREATED"
	EventUploadProgress  MediaEventType = "UPLOAD_PROGRESS"
	EventUploadCompleted MediaEventType = "UPLOAD_COMPLETED"
	EventUploadAborted   MediaEventType = "UPLOAD_ABORTED"
	EventUploadFailed    MediaEventType = "UPLOAD_FAILED"

	// Processing lifecycle events
	EventScanStarted      MediaEventType = "SCAN_STARTED"
	EventScanCompleted    MediaEventType = "SCAN_COMPLETED"
	EventScanFailed       MediaEventType = "SCAN_FAILED"
	EventMediaQuarantined MediaEventType = "MEDIA_QUARANTINED"

	EventProcessingStarted   MediaEventType = "PROCESSING_STARTED"
	EventProcessingProgress  MediaEventType = "PROCESSING_PROGRESS"
	EventProcessingCompleted MediaEventType = "PROCESSING_COMPLETED"
	EventProcessingFailed    MediaEventType = "PROCESSING_FAILED"

	EventMediaReady MediaEventType = "MEDIA_READY"

	// Delete lifecycle events
	EventDeleteRequested MediaEventType = "DELETE_REQUESTED"
	EventDeleteCompleted MediaEventType = "DELETE_COMPLETED"
	EventDeleteFailed    MediaEventType = "DELETE_FAILED"
)

// MediaEvent is the payload pushed to WebSocket clients for real-time updates.
// Every field is intentionally serialisable to JSON so the browser can consume it directly.
type MediaEvent struct {
	// EventType identifies the lifecycle step that triggered this notification.
	EventType MediaEventType `json:"eventType"`

	// MediaID is the unique identifier of the media item being acted upon.
	MediaID string `json:"mediaId"`

	// UploadID is populated for upload-phase events.
	UploadID string `json:"uploadId,omitempty"`

	// Status is the current media status string (e.g. "SCANNING", "READY").
	Status string `json:"status,omitempty"`

	// Progress is a 0-100 integer representing completion percentage.
	Progress int `json:"progress,omitempty"`

	// Error contains a human-readable error description when the event signals failure.
	Error string `json:"error,omitempty"`

	// TraceID allows the client to correlate WebSocket events with backend traces.
	TraceID string `json:"traceId,omitempty"`

	// Timestamp is the UTC time at which the event was generated.
	Timestamp time.Time `json:"timestamp"`
}

// Notifier is the port that workers use to push real-time updates to connected clients.
// The concrete implementation (Redis Pub/Sub → WebSocket Hub) is entirely hidden
// behind this interface, keeping workers independent of the transport mechanism.
// Contract:
//   - Notify must be non-blocking for the caller. If the delivery is asynchronous,
//     the implementation must handle back-pressure internally.
//   - Notify must never panic; errors should be logged internally.
//   - Close must drain any pending notifications before returning.
type Notifier interface {
	// Notify sends a MediaEvent to all active connections belonging to userID.
	Notify(ctx context.Context, userID string, event MediaEvent) error

	// Close gracefully shuts down the notifier.
	Close() error
}
