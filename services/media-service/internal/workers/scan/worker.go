// Package scan implements the malware scanning worker.
// It consumes media.upload.completed events and simulates ClamAV scanning.
// In production, replace scanWithClamAV with a real TCP/Unix socket call to the ClamAV daemon.
package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"

	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// uploadCompletedPayload mirrors the media.upload.completed event payload.
type uploadCompletedPayload struct {
	MediaID     string `json:"mediaId"`
	UserID      string `json:"userId"`
	ObjectKey   string `json:"objectKey"`
	ContentType string `json:"contentType"`
	MediaType   string `json:"mediaType"`
}

// Worker scans uploaded objects for malware before allowing processing.
type Worker struct {
	mediaRepo ports.MediaRepository
	storage   ports.ObjectStorage
	publisher ports.EventPublisher
}

// NewWorker creates a new scan worker.
func NewWorker(
	mediaRepo ports.MediaRepository,
	storage ports.ObjectStorage,
	publisher ports.EventPublisher,
) *Worker {
	return &Worker{
		mediaRepo: mediaRepo,
		storage:   storage,
		publisher: publisher,
	}
}

// Handle processes a single Kafka message from the media.upload.completed topic.
func (w *Worker) Handle(ctx context.Context, msg kafkago.Message) error {
	var env kafkaadapter.EventEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		// Unparseable messages are permanent failures — no retry benefit.
		return fmt.Errorf("%w: unmarshal scan event: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	var payload uploadCompletedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return fmt.Errorf("%w: unmarshal scan payload: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	return w.scan(ctx, payload, env.TraceID)
}

// scan transitions the media to SCANNING, runs the malware check, then transitions
// to PROCESSING (clean) or QUARANTINED (infected).
func (w *Worker) scan(ctx context.Context, payload uploadCompletedPayload, traceID string) error {
	slog.Info("Scan worker: starting", "mediaId", payload.MediaID)

	// UPLOADED → SCANNING
	if err := w.mediaRepo.UpdateStatus(ctx, payload.MediaID, domain.MediaStatusUploaded, domain.MediaStatusScanning); err != nil {
		// If we can't transition, another worker already picked this up — skip.
		slog.Warn("Scan worker: failed to transition to SCANNING (already taken?)", "mediaId", payload.MediaID, "error", err)
		return nil
	}

	// Publish scan.started event
	w.publishEvent(ctx, kafkaadapter.TopicScanStarted, payload.MediaID, payload.UserID, traceID, map[string]interface{}{
		"mediaId": payload.MediaID,
		"userId":  payload.UserID,
	})

	// Perform the actual scan (ClamAV TCP/Unix socket in production).
	infected, threat, err := w.scanWithClamAV(ctx, payload.ObjectKey)
	if err != nil {
		slog.Error("Scan worker: scan engine error — treating as retriable", "mediaId", payload.MediaID, "error", err)
		return fmt.Errorf("scan engine error: %w", err) // retriable
	}

	if infected {
		slog.Warn("Scan worker: malware detected", "mediaId", payload.MediaID, "threat", threat)

		// Transition to QUARANTINED.
		if tErr := w.mediaRepo.UpdateStatus(ctx, payload.MediaID, domain.MediaStatusScanning, domain.MediaStatusQuarantined); tErr != nil {
			slog.Error("Scan worker: failed to quarantine", "mediaId", payload.MediaID, "error", tErr)
		}

		// Publish scan.failed event.
		w.publishEvent(ctx, kafkaadapter.TopicScanFailed, payload.MediaID, payload.UserID, traceID, map[string]interface{}{
			"mediaId":  payload.MediaID,
			"userId":   payload.UserID,
			"infected": true,
			"threat":   threat,
		})
		return nil // quarantined — not a retriable error
	}

	// SCANNING → PROCESSING
	if err := w.mediaRepo.UpdateStatus(ctx, payload.MediaID, domain.MediaStatusScanning, domain.MediaStatusProcessing); err != nil {
		return fmt.Errorf("transition to PROCESSING: %w", err)
	}

	// Publish scan.completed event (triggers image/video/compression workers).
	w.publishEvent(ctx, kafkaadapter.TopicScanCompleted, payload.MediaID, payload.UserID, traceID, map[string]interface{}{
		"mediaId":     payload.MediaID,
		"userId":      payload.UserID,
		"objectKey":   payload.ObjectKey,
		"contentType": payload.ContentType,
		"mediaType":   payload.MediaType,
	})

	slog.Info("Scan worker: clean — dispatched to processing", "mediaId", payload.MediaID)
	return nil
}

// scanWithClamAV connects to the ClamAV daemon and scans the S3 object.
// Production implementation should stream the object body via clamav-go or equivalent.
// This stub always returns clean — replace with real implementation.
func (w *Worker) scanWithClamAV(ctx context.Context, objectKey string) (infected bool, threat string, err error) {
	// TODO: Stream object from S3 → ClamAV daemon via TCP socket (port 3310).
	// Example with clamav-go:
	//   reader, err := w.storage.GetObject(ctx, objectKey)
	//   result, err := clamav.ScanReader(reader)
	//   return result.Infected, result.Threat, err
	slog.Debug("Scan worker: ClamAV stub — treating as clean", "objectKey", objectKey)
	return false, "", nil
}

// publishEvent fires a Kafka event via the outbox publisher.
func (w *Worker) publishEvent(ctx context.Context, topic, mediaID, userID, traceID string, payload map[string]interface{}) {
	eventID := uuid.NewString()
	data, _ := kafkaadapter.MarshalEnvelope(eventID, topic, traceID, payload)
	if err := w.publisher.Publish(ctx, topic, mediaID, data, traceID); err != nil {
		slog.Warn("Scan worker: failed to publish event", "topic", topic, "mediaId", mediaID, "error", err)
	}
}
