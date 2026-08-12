package image

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"strings"

	_ "image/gif"
	_ "golang.org/x/image/webp"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/google/uuid"
	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
	"time"
	"bytes"
	"golang.org/x/image/draw"
)

// uploadCompletedPayload is the event payload for media.upload.completed.
type uploadCompletedPayload struct {
	MediaID     string `json:"mediaId"`
	UserID      string `json:"userId"`
	ObjectKey   string `json:"objectKey"`
	ContentType string `json:"contentType"`
	MediaType   string `json:"mediaType"`
}

// Worker processes image media after upload.
// It generates a thumbnail and an optimised version, then uploads both to S3.
type Worker struct {
	mediaRepo   ports.MediaRepository
	versionRepo ports.VersionRepository
	storage     ports.ObjectStorage
	cache       ports.Cache
	publisher   ports.EventPublisher
}

// NewWorker creates a new image processing worker.
func NewWorker(
	mediaRepo ports.MediaRepository,
	versionRepo ports.VersionRepository,
	storage ports.ObjectStorage,
	cache ports.Cache,
	publisher ports.EventPublisher,
) *Worker {
	return &Worker{
		mediaRepo:   mediaRepo,
		versionRepo: versionRepo,
		storage:     storage,
		cache:       cache,
		publisher:   publisher,
	}
}

// Handle processes a single Kafka message for image processing.
func (w *Worker) Handle(ctx context.Context, msg kafkago.Message) error {
	var env kafkaadapter.EventEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		return fmt.Errorf("%w: unmarshal image event: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	var payload uploadCompletedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return fmt.Errorf("%w: unmarshal image payload: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	// Only process image types
	if payload.MediaType != string(domain.MediaTypeImage) {
		return nil // not our job — skip silently
	}

	return w.processImage(ctx, payload, env.TraceID)
}

// processImage downloads the original, generates thumbnail + optimised versions, uploads to S3.
// Pre-condition: media is already in PROCESSING state (set by the scan worker).
func (w *Worker) processImage(ctx context.Context, payload uploadCompletedPayload, traceID string) error {
	slog.Info("Image worker: processing", "mediaId", payload.MediaID)

	// Report progress via Redis TTL key (expires automatically — no separate cleanup needed)
	_ = w.cache.SetProcessingProgress(ctx, payload.MediaID, 10, 2*time.Hour)

	// Download original image
	body, err := w.storage.GetObject(ctx, payload.ObjectKey)
	if err != nil {
		return fmt.Errorf("download original image: %w", err)
	}
	defer body.Close()

	imgData, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read image data: %w", err)
	}

	src, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return fmt.Errorf("%w: decode image: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	_ = w.cache.SetProcessingProgress(ctx, payload.MediaID, 30, 2*time.Hour)

	// Generate thumbnail (200x200)
	if err := w.generateAndUpload(ctx, payload, traceID, src, 200, 200, domain.VersionTypeThumbnail); err != nil {
		slog.Error("Image worker: thumbnail generation failed", "mediaId", payload.MediaID, "error", err)
	}

	_ = w.cache.SetProcessingProgress(ctx, payload.MediaID, 60, 2*time.Hour)

	// Generate medium version (800x600)
	if err := w.generateAndUpload(ctx, payload, traceID, src, 800, 600, domain.VersionTypeMedium); err != nil {
		slog.Error("Image worker: medium generation failed", "mediaId", payload.MediaID, "error", err)
	}

	_ = w.cache.SetProcessingProgress(ctx, payload.MediaID, 100, 2*time.Hour)

	// Transition media to READY
	if err := w.mediaRepo.UpdateStatus(ctx, payload.MediaID, domain.MediaStatusProcessing, domain.MediaStatusReady); err != nil {
		return fmt.Errorf("transition to READY: %w", err)
	}

	// Publish media.ready event
	eventBytes, _ := kafkaadapter.MarshalEnvelope(uuid.NewString(), kafkaadapter.TopicMediaReady, traceID,
		map[string]interface{}{"mediaId": payload.MediaID, "userId": payload.UserID})
	_ = w.publisher.Publish(ctx, kafkaadapter.TopicMediaReady, payload.MediaID, eventBytes, traceID)

	slog.Info("Image worker: processing complete", "mediaId", payload.MediaID)
	return nil
}

// generateAndUpload resizes the image and uploads it to S3 as a new version via PutObject.
func (w *Worker) generateAndUpload(ctx context.Context, payload uploadCompletedPayload, traceID string, src image.Image, maxW, maxH int, versionType domain.VersionType) error {
	resized := resizeImage(src, maxW, maxH)

	var buf bytes.Buffer
	var contentType string
	if strings.Contains(payload.ContentType, "png") {
		contentType = "image/png"
		if err := png.Encode(&buf, resized); err != nil {
			return fmt.Errorf("encode PNG %s: %w", versionType, err)
		}
	} else {
		contentType = "image/jpeg"
		if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 85}); err != nil {
			return fmt.Errorf("encode JPEG %s: %w", versionType, err)
		}
	}

	versionID := uuid.NewString()
	ext := ".jpg"
	if contentType == "image/png" {
		ext = ".png"
	}
	objectKey := buildVersionKey(payload.ObjectKey, string(versionType), ext)

	// Upload processed rendition directly (not multipart — processed images are small)
	data := buf.Bytes()
	if err := w.storage.PutObject(ctx, objectKey, contentType, data); err != nil {
		return fmt.Errorf("upload version %s to S3: %w", versionType, err)
	}

	if err := w.versionRepo.Create(ctx, &domain.MediaVersion{
		VersionID:   versionID,
		MediaID:     payload.MediaID,
		VersionType: versionType,
		ObjectKey:   objectKey,
		ContentType: contentType,
		Size:        int64(len(data)),
		Width:       int32(resized.Bounds().Dx()),
		Height:      int32(resized.Bounds().Dy()),
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("persist version metadata %s: %w", versionType, err)
	}

	return nil
}

// resizeImage scales an image to fit within maxW x maxH maintaining aspect ratio.
func resizeImage(src image.Image, maxW, maxH int) image.Image {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	ratio := 1.0
	if float64(srcW)/float64(maxW) > float64(srcH)/float64(maxH) {
		ratio = float64(maxW) / float64(srcW)
	} else {
		ratio = float64(maxH) / float64(srcH)
	}

	dstW := int(float64(srcW) * ratio)
	dstH := int(float64(srcH) * ratio)

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

// buildVersionKey constructs the S3 key for a processed version.
func buildVersionKey(originalKey, versionType, ext string) string {
	// Replace "/original/" with "/{versionType}/"
	parts := strings.SplitN(originalKey, "/original/", 2)
	if len(parts) == 2 {
		baseName := strings.TrimSuffix(parts[1], fmt.Sprintf(".%s", strings.Split(parts[1], ".")[len(strings.Split(parts[1], "."))-1]))
		return fmt.Sprintf("%s/%s/%s%s", parts[0], versionType, baseName, ext)
	}
	return originalKey + "_" + versionType + ext
}
