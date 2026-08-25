// Package metadata implements the metadata extraction worker.
// It consumes media.scan.completed events and extracts real technical metadata:
//   - Images: Go stdlib image.DecodeConfig (width, height, format)
//   - Videos: ffprobe JSON output (width, height, duration, codec)
//
// Results are persisted to DynamoDB as a MediaVersion record and a processing event is published.
package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"

	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// scanCompletedPayload mirrors the media.scan.completed event payload.
type scanCompletedPayload struct {
	MediaID     string `json:"mediaId"`
	UserID      string `json:"userId"`
	ObjectKey   string `json:"objectKey"`
	ContentType string `json:"contentType"`
	MediaType   string `json:"mediaType"`
}

// Worker extracts technical metadata from uploaded files after malware scanning.
type Worker struct {
	mediaRepo   ports.MediaRepository
	versionRepo ports.VersionRepository
	storage     ports.ObjectStorage
	publisher   ports.EventPublisher
	extractors  *extractorRegistry
}

// NewWorker creates a new metadata extraction worker.
func NewWorker(
	mediaRepo ports.MediaRepository,
	versionRepo ports.VersionRepository,
	storage ports.ObjectStorage,
	publisher ports.EventPublisher,
) *Worker {
	return &Worker{
		mediaRepo:   mediaRepo,
		versionRepo: versionRepo,
		storage:     storage,
		publisher:   publisher,
		extractors:  BuildExtractorRegistry(storage),
	}
}

// Handle processes a single Kafka message from media.scan.completed.
func (w *Worker) Handle(ctx context.Context, msg kafkago.Message) error {
	var env kafkaadapter.EventEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		return fmt.Errorf("%w: unmarshal metadata event: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	var payload scanCompletedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return fmt.Errorf("%w: unmarshal metadata payload: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	return w.extract(ctx, payload, env.TraceID)
}

// extract dispatches to the correct extractor based on media type.
func (w *Worker) extract(ctx context.Context, payload scanCompletedPayload, traceID string) error {
	slog.Info("Metadata worker: extracting", "mediaId", payload.MediaID, "mediaType", payload.MediaType)

	extractor, ok := w.extractors.get(domain.MediaType(payload.MediaType))
	if !ok {
		slog.Warn("No metadata extractor for media type", "mediaType", payload.MediaType, "mediaId", payload.MediaID)
		return w.persistEmptyMeta(ctx, payload, traceID)
	}

	meta, err := extractor.Extract(ctx, payload.ObjectKey)
	if err != nil {
		slog.Warn("Metadata extraction failed", "mediaId", payload.MediaID, "mediaType", payload.MediaType, "error", err)
		return w.persistEmptyMeta(ctx, payload, traceID)
	}

	return w.persistMeta(ctx, payload, traceID, meta)
}

func (w *Worker) persistMeta(ctx context.Context, payload scanCompletedPayload, traceID string, meta extractedMeta) error {
	// Persist metadata as a special "metadata" version record (no new S3 object).
	_ = w.versionRepo.Create(ctx, &domain.MediaVersion{
		VersionID:   uuid.NewString(),
		MediaID:     payload.MediaID,
		VersionType: domain.VersionType("metadata"),
		ObjectKey:   payload.ObjectKey,
		ContentType: payload.ContentType,
		Width:       meta.Width,
		Height:      meta.Height,
		DurationMS:  meta.DurationMS,
		Checksum:    meta.Format,
		CreatedAt:   time.Now().UTC(),
	})

	// Publish processing.completed
	eventData, _ := kafkaadapter.MarshalEnvelope(uuid.NewString(), kafkaadapter.TopicProcessingCompleted, traceID,
		map[string]interface{}{
			"mediaId":    payload.MediaID,
			"userId":     payload.UserID,
			"workerType": "metadata",
			"width":      meta.Width,
			"height":     meta.Height,
			"durationMs": meta.DurationMS,
			"format":     meta.Format,
		})
	_ = w.publisher.Publish(ctx, kafkaadapter.TopicProcessingCompleted, payload.MediaID, eventData, traceID)

	slog.Info("Metadata worker: done",
		"mediaId", payload.MediaID,
		"width", meta.Width,
		"height", meta.Height,
		"durationMs", meta.DurationMS,
		"format", meta.Format,
	)
	return nil
}

func (w *Worker) persistEmptyMeta(ctx context.Context, payload scanCompletedPayload, traceID string) error {
	meta := extractedMeta{}
	return w.persistMeta(ctx, payload, traceID, meta)
}