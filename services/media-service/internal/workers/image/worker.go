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
	"os"
	"os/exec"
	"strings"

	_ "golang.org/x/image/webp"
	_ "image/gif"

	"bytes"
	sharednats "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/nats"
	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
	"golang.org/x/image/draw"
	"time"
)

// uploadCompletedPayload is the event payload for media.upload.completed.
type uploadCompletedPayload struct {
	MediaID     string `json:"mediaId"`
	UserID      string `json:"userId"`
	FileName    string `json:"fileName"`
	ObjectKey   string `json:"objectKey"`
	Size        int64  `json:"size"`
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
	realtimePub ports.RealtimePublisher
}

// NewWorker creates a new image processing worker.
func NewWorker(
	mediaRepo ports.MediaRepository,
	versionRepo ports.VersionRepository,
	storage ports.ObjectStorage,
	cache ports.Cache,
	publisher ports.EventPublisher,
	realtimePub ports.RealtimePublisher,
) *Worker {
	return &Worker{
		mediaRepo:   mediaRepo,
		versionRepo: versionRepo,
		storage:     storage,
		cache:       cache,
		publisher:   publisher,
		realtimePub: realtimePub,
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
	_ = w.realtimePub.PublishProgress(ctx, sharednats.RealtimeMediaProcessingProgress, map[string]interface{}{
		"mediaId": payload.MediaID,
		"userId":  payload.UserID,
		"stage":   "image",
		"percent": 10,
	})

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
		slog.Warn("Image worker: decode image failed, proceeding with original image as ready", "mediaId", payload.MediaID, "error", err)
	} else {
		_ = w.cache.SetProcessingProgress(ctx, payload.MediaID, 30, 2*time.Hour)
		_ = w.realtimePub.PublishProgress(ctx, sharednats.RealtimeMediaProcessingProgress, map[string]interface{}{
			"mediaId": payload.MediaID,
			"userId":  payload.UserID,
			"stage":   "image",
			"percent": 30,
		})

		// Generate thumbnail (200x200)
		if err := w.generateAndUpload(ctx, payload, traceID, src, 200, 200, domain.VersionTypeThumbnail); err != nil {
			slog.Warn("Image worker: thumbnail generation failed (non-fatal)", "mediaId", payload.MediaID, "error", err)
		}

		_ = w.cache.SetProcessingProgress(ctx, payload.MediaID, 60, 2*time.Hour)
		_ = w.realtimePub.PublishProgress(ctx, sharednats.RealtimeMediaProcessingProgress, map[string]interface{}{
			"mediaId": payload.MediaID,
			"userId":  payload.UserID,
			"stage":   "image",
			"percent": 60,
		})

		// Generate medium version (800x600)
		if err := w.generateAndUpload(ctx, payload, traceID, src, 800, 600, domain.VersionTypeMedium); err != nil {
			slog.Warn("Image worker: medium generation failed (non-fatal)", "mediaId", payload.MediaID, "error", err)
		}

		// Modern formats (non-fatal)
		for _, rendition := range []struct {
			format      string
			contentType string
			codec       string
			versionType domain.VersionType
		}{
			{format: "webp", contentType: "image/webp", codec: "libwebp", versionType: domain.VersionTypeWebP},
			{format: "avif", contentType: "image/avif", codec: "libaom-av1", versionType: domain.VersionTypeAVIF},
		} {
			if err := w.generateAndUploadModernFormat(ctx, payload, traceID, src, 800, 600, rendition.format, rendition.contentType, rendition.codec, rendition.versionType); err != nil {
				slog.Warn("Image worker: modern format rendition failed, continuing", "format", rendition.format, "error", err)
			}
		}
	}

	_ = w.cache.SetProcessingProgress(ctx, payload.MediaID, 100, 2*time.Hour)
	_ = w.realtimePub.PublishProgress(ctx, sharednats.RealtimeMediaProcessingProgress, map[string]interface{}{
		"mediaId": payload.MediaID,
		"userId":  payload.UserID,
		"stage":   "image",
		"percent": 100,
	})

	// Transition media to READY
	if err := w.mediaRepo.UpdateStatus(ctx, payload.MediaID, domain.MediaStatusProcessing, domain.MediaStatusReady); err != nil {
		return fmt.Errorf("transition to READY: %w", err)
	}

	// Publish media.ready event to Kafka
	fileName := payload.FileName
	size := payload.Size
	if fileName == "" {
		if m, err := w.mediaRepo.GetByID(ctx, payload.MediaID); err == nil && m != nil {
			fileName = m.FileName
			if size == 0 {
				size = m.Size
			}
		}
	}
	eventBytes, _ := kafkaadapter.MarshalEnvelope(uuid.NewString(), kafkaadapter.TopicMediaReady, traceID,
		map[string]interface{}{
			"mediaId":     payload.MediaID,
			"userId":      payload.UserID,
			"fileName":    fileName,
			"contentType": payload.ContentType,
			"mediaType":   payload.MediaType,
			"size":        size,
		})
	_ = w.publisher.Publish(ctx, kafkaadapter.TopicMediaReady, payload.MediaID, eventBytes, traceID)

	// Publish media ready to NATS for realtime WebSocket
	_ = w.realtimePub.PublishProgress(ctx, sharednats.RealtimeMediaReady, map[string]interface{}{
		"mediaId":   payload.MediaID,
		"userId":    payload.UserID,
		"mediaType": payload.MediaType,
	})

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

// generateAndUploadModernFormat encodes a resized PNG through FFmpeg into a
// modern browser format and persists its object and metadata atomically enough
// for the idempotent worker retry path.
func (w *Worker) generateAndUploadModernFormat(ctx context.Context, payload uploadCompletedPayload, traceID string, src image.Image, maxW, maxH int, format, contentType, codec string, versionType domain.VersionType) error {
	resized := resizeImage(src, maxW, maxH)
	var input bytes.Buffer
	if err := png.Encode(&input, resized); err != nil {
		return fmt.Errorf("encode intermediate PNG: %w", err)
	}

	ffmpegPath := os.Getenv("FFMPEG_PATH")
	if ffmpegPath == "" {
		var err error
		ffmpegPath, err = exec.LookPath("ffmpeg")
		if err != nil {
			return fmt.Errorf("ffmpeg not available: %w", err)
		}
	}
	cmd := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-loglevel", "error", "-f", "png_pipe", "-i", "pipe:0", "-frames:v", "1", "-c:v", codec, "-f", format, "pipe:1")
	cmd.Stdin = bytes.NewReader(input.Bytes())
	var output bytes.Buffer
	cmd.Stdout = &output
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg %s: %w (%s)", format, err, strings.TrimSpace(stderr.String()))
	}
	data := output.Bytes()
	if len(data) == 0 {
		return fmt.Errorf("ffmpeg %s returned empty output", format)
	}

	objectKey := buildVersionKey(payload.ObjectKey, string(versionType), "."+format)
	if err := w.storage.PutObject(ctx, objectKey, contentType, data); err != nil {
		return fmt.Errorf("upload %s to S3: %w", format, err)
	}
	if err := w.versionRepo.Create(ctx, &domain.MediaVersion{
		VersionID: uuid.NewString(), MediaID: payload.MediaID, VersionType: versionType,
		ObjectKey: objectKey, ContentType: contentType, Size: int64(len(data)),
		Width: int32(resized.Bounds().Dx()), Height: int32(resized.Bounds().Dy()), CreatedAt: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("persist %s metadata: %w", format, err)
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
