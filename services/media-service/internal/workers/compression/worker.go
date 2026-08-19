// Package compression implements the file compression worker.
// It consumes media.scan.completed events and compresses text-based files using gzip.
//
// Compression policy (matches architecture spec §31):
//   - Compress: text/*, application/json, application/xml, text/csv, text/plain, application/pdf
//   - Skip: already-compressed formats (JPEG, PNG, MP4, MKV, ZIP, etc.)
//
// The original file is preserved in S3 — only a new compressed version is created.
// This keeps bandwidth usage low while allowing clients to request the original if needed.
package compression

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"

	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// compressibleMIMEPrefixes lists MIME type prefixes that benefit from gzip compression.
// Formats already using lossy or lossless compression are excluded.
var compressibleMIMEPrefixes = []string{
	"text/",
	"application/json",
	"application/xml",
	"application/javascript",
	"application/x-www-form-urlencoded",
	"application/pdf",
	"application/rtf",
	"application/csv",
}

// scanCompletedPayload mirrors the media.scan.completed event payload.
type scanCompletedPayload struct {
	MediaID     string `json:"mediaId"`
	UserID      string `json:"userId"`
	ObjectKey   string `json:"objectKey"`
	ContentType string `json:"contentType"`
	MediaType   string `json:"mediaType"`
}

// Worker compresses eligible files after they pass malware scanning.
type Worker struct {
	mediaRepo   ports.MediaRepository
	versionRepo ports.VersionRepository
	storage     ports.ObjectStorage
	publisher   ports.EventPublisher
}

// NewWorker creates a new compression worker.
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
	}
}

// Handle processes a single Kafka message from media.scan.completed.
func (w *Worker) Handle(ctx context.Context, msg kafkago.Message) error {
	var env kafkaadapter.EventEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		return fmt.Errorf("%w: unmarshal compression event: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	var payload scanCompletedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return fmt.Errorf("%w: unmarshal compression payload: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	// Skip non-compressible media types (images and videos are handled by their own workers).
	if !isCompressible(payload.ContentType) {
		return nil
	}

	return w.compress(ctx, payload, env.TraceID)
}

// compress downloads the original file, gzips it, uploads the compressed version to S3,
// and saves the version record to DynamoDB.
func (w *Worker) compress(ctx context.Context, payload scanCompletedPayload, traceID string) error {
	start := time.Now()
	slog.Info("Compression worker: starting", "mediaId", payload.MediaID, "contentType", payload.ContentType)

	// Download original from S3
	body, err := w.storage.GetObject(ctx, payload.ObjectKey)
	if err != nil {
		return fmt.Errorf("download original for compression: %w", err)
	}
	defer body.Close()

	original, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read original bytes: %w", err)
	}

	// Gzip compress
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("create gzip writer: %w", err)
	}
	if _, err := gz.Write(original); err != nil {
		return fmt.Errorf("gzip write: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("gzip close: %w", err)
	}

	compressed := buf.Bytes()
	ratio := float64(len(compressed)) / float64(len(original))
	slog.Info("Compression worker: compressed",
		"mediaId", payload.MediaID,
		"originalBytes", len(original),
		"compressedBytes", len(compressed),
		"ratio", fmt.Sprintf("%.2f", ratio),
	)

	// Skip if compression made the file larger (e.g., already compressed PDF)
	if len(compressed) >= len(original) {
		slog.Info("Compression worker: skipping — no size reduction", "mediaId", payload.MediaID)
		return nil
	}

	// Upload compressed version to S3
	compressedKey := compressedObjectKey(payload.ObjectKey)
	if err := w.storage.PutObject(ctx, compressedKey, "application/gzip", compressed); err != nil {
		return fmt.Errorf("upload compressed version: %w", err)
	}

	// Save version record
	if err := w.versionRepo.Create(ctx, &domain.MediaVersion{
		VersionID:   uuid.NewString(),
		MediaID:     payload.MediaID,
		VersionType: domain.VersionTypeCompressed,
		ObjectKey:   compressedKey,
		ContentType: "application/gzip",
		Size:        int64(len(compressed)),
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		slog.Warn("Compression worker: failed to save version record", "mediaId", payload.MediaID, "error", err)
	}

	// Publish processing.completed event
	eventData, _ := kafkaadapter.MarshalEnvelope(uuid.NewString(), kafkaadapter.TopicProcessingCompleted, traceID,
		map[string]interface{}{
			"mediaId":     payload.MediaID,
			"userId":      payload.UserID,
			"workerType":  "compression",
			"originalSize": len(original),
			"newSize":     len(compressed),
		})
	_ = w.publisher.Publish(ctx, kafkaadapter.TopicProcessingCompleted, payload.MediaID, eventData, traceID)

	slog.Info("Compression worker: done",
		"mediaId", payload.MediaID,
		"durationMs", time.Since(start).Milliseconds(),
	)
	return nil
}

// isCompressible returns true if the MIME type benefits from gzip compression.
func isCompressible(contentType string) bool {
	// Strip parameters (e.g. "text/html; charset=utf-8" → "text/html")
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	for _, prefix := range compressibleMIMEPrefixes {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}
	return false
}

// compressedObjectKey derives the S3 key for the compressed version.
func compressedObjectKey(originalKey string) string {
	return strings.TrimSuffix(originalKey, "/") + ".gz"
}
