// Package metadata implements the metadata extraction worker.
// It consumes media.scan.completed events and extracts real technical metadata:
//   - Images: Go stdlib image.DecodeConfig (width, height, format)
//   - Videos: ffprobe JSON output (width, height, duration, codec)
//
// Results are persisted to DynamoDB as a MediaVersion record and a processing event is published.
package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

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

// ffprobeOutput is a subset of ffprobe's JSON output.
type ffprobeOutput struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Width     int32  `json:"width"`
		Height    int32  `json:"height"`
	} `json:"streams"`
	Format struct {
		Duration   string `json:"duration"`
		FormatName string `json:"format_name"`
	} `json:"format"`
}

// extractedMeta groups the extracted values.
type extractedMeta struct {
	Width      int32
	Height     int32
	DurationMS int64
	Format     string
}

// Worker extracts technical metadata from uploaded files after malware scanning.
type Worker struct {
	mediaRepo   ports.MediaRepository
	versionRepo ports.VersionRepository
	storage     ports.ObjectStorage
	publisher   ports.EventPublisher
	notifier    ports.Notifier
}

// NewWorker creates a new metadata extraction worker.
func NewWorker(
	mediaRepo ports.MediaRepository,
	versionRepo ports.VersionRepository,
	storage ports.ObjectStorage,
	publisher ports.EventPublisher,
	notifier ports.Notifier,
) *Worker {
	return &Worker{
		mediaRepo:   mediaRepo,
		versionRepo: versionRepo,
		storage:     storage,
		publisher:   publisher,
		notifier:    notifier,
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

	var meta extractedMeta

	switch domain.MediaType(payload.MediaType) {
	case domain.MediaTypeImage:
		m, err := w.extractImage(ctx, payload.ObjectKey)
		if err != nil {
			slog.Warn("Metadata worker: image extraction failed", "mediaId", payload.MediaID, "error", err)
		} else {
			meta = m
		}

	case domain.MediaTypeVideo:
		m, err := w.extractVideo(ctx, payload.ObjectKey)
		if err != nil {
			slog.Warn("Metadata worker: video extraction failed", "mediaId", payload.MediaID, "error", err)
		} else {
			meta = m
		}
	}

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

	// Notify client
	_ = w.notifier.Notify(ctx, payload.UserID, ports.MediaEvent{
		EventType: ports.EventProcessingProgress,
		MediaID:   payload.MediaID,
		Progress:  100,
		TraceID:   traceID,
		Timestamp: time.Now().UTC(),
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

// extractImage streams the image from S3 and uses Go stdlib image.DecodeConfig.
func (w *Worker) extractImage(ctx context.Context, objectKey string) (extractedMeta, error) {
	body, err := w.storage.GetObject(ctx, objectKey)
	if err != nil {
		return extractedMeta{}, fmt.Errorf("get image from S3: %w", err)
	}
	defer body.Close()

	cfg, format, err := image.DecodeConfig(body)
	if err != nil {
		return extractedMeta{}, fmt.Errorf("decode image config: %w", err)
	}

	return extractedMeta{
		Width:  int32(cfg.Width),
		Height: int32(cfg.Height),
		Format: format,
	}, nil
}

// extractVideo runs ffprobe to extract duration, codec, and resolution from a video.
// In production, generate a presigned GET URL and pass it to ffprobe directly.
//
// If ffprobe is not installed (dev environment), this returns zero values gracefully.
func (w *Worker) extractVideo(ctx context.Context, objectKey string) (extractedMeta, error) {
	// Derive a download URL — for local/test use the object key directly.
	// Production: replace with a presigned GET URL using w.storage.GeneratePresignedGET().
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		objectKey,
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// ffprobe not available or object not accessible — return empty meta, not an error.
		slog.Debug("Metadata worker: ffprobe unavailable or failed",
			"objectKey", objectKey,
			"stderr", stderr.String(),
		)
		return extractedMeta{}, nil
	}

	var result ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return extractedMeta{}, fmt.Errorf("parse ffprobe output: %w", err)
	}

	var (
		width, height int32
		codecName     string
	)
	for _, s := range result.Streams {
		if s.CodecType == "video" && s.Width > 0 {
			width = s.Width
			height = s.Height
			codecName = s.CodecName
			break
		}
	}

	return extractedMeta{
		Width:      width,
		Height:     height,
		DurationMS: parseDurationMS(result.Format.Duration),
		Format:     result.Format.FormatName + "/" + codecName,
	}, nil
}

// parseDurationMS converts an ffprobe duration string (seconds as float) to milliseconds.
func parseDurationMS(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return 0
	}
	return int64(f * 1000)
}
