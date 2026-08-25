// Package video implements the video transcoding worker.
// It consumes media.scan.completed events (filtered to VIDEO type) and uses FFmpeg
// to produce 360p / 720p / 1080p / HLS renditions and a thumbnail.
//
// Production requirement: FFmpeg must be installed in the container image.
// Dockerfile should include: `apt-get install -y ffmpeg`
package video

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"

	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	sharednats "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/nats"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// scanCompletedPayload is the scan.completed event payload.
type scanCompletedPayload struct {
	MediaID     string `json:"mediaId"`
	UserID      string `json:"userId"`
	FileName    string `json:"fileName"`
	ObjectKey   string `json:"objectKey"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	MediaType   string `json:"mediaType"`
}

// videoProfile defines a single transcoding output rendition.
type videoProfile struct {
	Name        string
	Height      int
	Bitrate     string
	VersionType domain.VersionType
}

// standardProfiles matches YouTube's adaptive bitrate ladder.
var standardProfiles = []videoProfile{
	{Name: "360p", Height: 360, Bitrate: "800k", VersionType: domain.VersionType360p},
	{Name: "720p", Height: 720, Bitrate: "2500k", VersionType: domain.VersionType720p},
	{Name: "1080p", Height: 1080, Bitrate: "5000k", VersionType: domain.VersionType1080p},
}

// Worker processes video media after upload and malware scanning.
type Worker struct {
	mediaRepo      ports.MediaRepository
	versionRepo    ports.VersionRepository
	storage        ports.ObjectStorage
	publisher      ports.EventPublisher
	realtimePub    ports.RealtimePublisher
}

// NewWorker creates a new video transcoding worker.
func NewWorker(
	mediaRepo ports.MediaRepository,
	versionRepo ports.VersionRepository,
	storage ports.ObjectStorage,
	publisher ports.EventPublisher,
	realtimePub ports.RealtimePublisher,
) *Worker {
	return &Worker{
		mediaRepo:   mediaRepo,
		versionRepo: versionRepo,
		storage:     storage,
		publisher:   publisher,
		realtimePub: realtimePub,
	}
}

// Handle processes a single Kafka message from media.scan.completed.
func (w *Worker) Handle(ctx context.Context, msg kafkago.Message) error {
	var env kafkaadapter.EventEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		return fmt.Errorf("%w: unmarshal video event: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	var payload scanCompletedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return fmt.Errorf("%w: unmarshal video payload: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	// Only process VIDEO types — other workers handle images and documents.
	if payload.MediaType != string(domain.MediaTypeVideo) {
		return nil
	}

	return w.processVideo(ctx, payload, env.TraceID)
}

// processVideo orchestrates FFmpeg transcoding and HLS generation.
func (w *Worker) processVideo(ctx context.Context, payload scanCompletedPayload, traceID string) error {
	slog.Info("Video worker: starting transcoding", "mediaId", payload.MediaID)

	// Transition to PROCESSING (scan worker already brought us here, but ensure idempotency).
	_ = w.mediaRepo.UpdateStatus(ctx, payload.MediaID, domain.MediaStatusScanning, domain.MediaStatusProcessing)

	_ = w.realtimePub.PublishProgress(ctx, sharednats.RealtimeMediaProcessingProgress, map[string]interface{}{
		"mediaId": payload.MediaID,
		"userId":  payload.UserID,
		"stage":   "video",
		"percent": 5,
	})

	// Create a temporary workspace for FFmpeg output.
	tmpDir, err := os.MkdirTemp("", "media-transcode-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download the original video from S3.
	origBody, err := w.storage.GetObject(ctx, payload.ObjectKey)
	if err != nil {
		return fmt.Errorf("download original video: %w", err)
	}
	inputPath := filepath.Join(tmpDir, "original.mp4")
	inputFile, err := os.Create(inputPath)
	if err != nil {
		origBody.Close()
		return fmt.Errorf("create temp input file: %w", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(origBody); err != nil {
		origBody.Close()
		inputFile.Close()
		return fmt.Errorf("read original video: %w", err)
	}
	origBody.Close()
	if _, err := inputFile.Write(buf.Bytes()); err != nil {
		inputFile.Close()
		return fmt.Errorf("write temp input file: %w", err)
	}
	inputFile.Close()

	_ = w.realtimePub.PublishProgress(ctx, sharednats.RealtimeMediaProcessingProgress, map[string]interface{}{
		"mediaId": payload.MediaID,
		"userId":  payload.UserID,
		"stage":   "video",
		"percent": 10,
	})

	// Generate thumbnail from first frame.
	if err := w.generateThumbnail(ctx, payload, traceID, inputPath, tmpDir); err != nil {
		slog.Warn("Video worker: thumbnail generation failed (non-fatal)", "mediaId", payload.MediaID, "error", err)
	}

	_ = w.realtimePub.PublishProgress(ctx, sharednats.RealtimeMediaProcessingProgress, map[string]interface{}{
		"mediaId": payload.MediaID,
		"userId":  payload.UserID,
		"stage":   "video",
		"percent": 15,
	})

	// Transcode each standard profile.
	allFailed := true
	for i, profile := range standardProfiles {
		_ = w.realtimePub.PublishProgress(ctx, sharednats.RealtimeMediaProcessingProgress, map[string]interface{}{
			"mediaId": payload.MediaID,
			"userId":  payload.UserID,
			"stage":   "video",
			"percent": 15 + (i * 25), // 15, 40, 65
		})

		if err := w.transcodeProfile(ctx, payload, traceID, inputPath, tmpDir, profile); err != nil {
			slog.Error("Video worker: transcoding failed for profile", "profile", profile.Name, "mediaId", payload.MediaID, "error", err)
			continue
		}
		allFailed = false
	}

	if allFailed {
		slog.Warn("Video worker: profile transcoding had failures, proceeding with original video", "mediaId", payload.MediaID)
	}

	_ = w.realtimePub.PublishProgress(ctx, sharednats.RealtimeMediaProcessingProgress, map[string]interface{}{
		"mediaId": payload.MediaID,
		"userId":  payload.UserID,
		"stage":   "video",
		"percent": 90,
	})

	// Generate HLS master playlist.
	if err := w.generateHLS(ctx, payload, traceID, inputPath, tmpDir); err != nil {
		slog.Warn("Video worker: HLS generation failed (non-fatal)", "mediaId", payload.MediaID, "error", err)
	}

	// Transition to READY.
	if err := w.mediaRepo.UpdateStatus(ctx, payload.MediaID, domain.MediaStatusProcessing, domain.MediaStatusReady); err != nil {
		return fmt.Errorf("transition to READY: %w", err)
	}

	// Publish media.ready.
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
	w.publishEvent(ctx, kafkaadapter.TopicMediaReady, payload.MediaID, payload.UserID, traceID, map[string]interface{}{
		"mediaId":     payload.MediaID,
		"userId":      payload.UserID,
		"fileName":    fileName,
		"contentType": payload.ContentType,
		"mediaType":   payload.MediaType,
		"size":        size,
	})

	// Publish media ready to NATS for realtime WebSocket
	_ = w.realtimePub.PublishProgress(ctx, sharednats.RealtimeMediaReady, map[string]interface{}{
		"mediaId":   payload.MediaID,
		"userId":    payload.UserID,
		"mediaType": payload.MediaType,
	})

	slog.Info("Video worker: transcoding complete", "mediaId", payload.MediaID)
	return nil
}

// generateThumbnail extracts a JPEG thumbnail from the first frame using FFmpeg.
func (w *Worker) generateThumbnail(ctx context.Context, payload scanCompletedPayload, traceID, inputPath, tmpDir string) error {
	thumbPath := filepath.Join(tmpDir, "thumbnail.jpg")
	args := []string{
		"-y", "-i", inputPath,
		"-vframes", "1",
		"-q:v", "2",
		thumbPath,
	}

	if err := runFFmpeg(ctx, args); err != nil {
		return fmt.Errorf("ffmpeg thumbnail: %w", err)
	}

	data, err := os.ReadFile(thumbPath)
	if err != nil {
		return fmt.Errorf("read thumbnail: %w", err)
	}

	objectKey := buildVersionKey(payload.ObjectKey, "thumbnail", ".jpg")
	if err := w.storage.PutObject(ctx, objectKey, "image/jpeg", data); err != nil {
		return fmt.Errorf("upload thumbnail: %w", err)
	}

	_ = w.versionRepo.Create(ctx, &domain.MediaVersion{
		VersionID:   uuid.NewString(),
		MediaID:     payload.MediaID,
		VersionType: domain.VersionTypeThumbnail,
		ObjectKey:   objectKey,
		ContentType: "image/jpeg",
		Size:        int64(len(data)),
		CreatedAt:   time.Now().UTC(),
	})

	return nil
}

// transcodeProfile runs FFmpeg for a single resolution profile.
func (w *Worker) transcodeProfile(ctx context.Context, payload scanCompletedPayload, traceID, inputPath, tmpDir string, profile videoProfile) error {
	outputName := fmt.Sprintf("%s.mp4", profile.Name)
	outputPath := filepath.Join(tmpDir, outputName)

	args := []string{
		"-y", "-i", inputPath,
		"-vf", fmt.Sprintf("scale=-2:%d", profile.Height),
		"-c:v", "libx264", "-preset", "fast", "-b:v", profile.Bitrate,
		"-c:a", "aac", "-b:a", "128k",
		"-movflags", "+faststart", // enable progressive download
		outputPath,
	}

	if err := runFFmpeg(ctx, args); err != nil {
		return fmt.Errorf("ffmpeg transcode %s: %w", profile.Name, err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("read transcoded output %s: %w", profile.Name, err)
	}

	objectKey := buildVersionKey(payload.ObjectKey, profile.Name, ".mp4")
	if err := w.storage.PutObject(ctx, objectKey, "video/mp4", data); err != nil {
		return fmt.Errorf("upload %s: %w", profile.Name, err)
	}

	_ = w.versionRepo.Create(ctx, &domain.MediaVersion{
		VersionID:   uuid.NewString(),
		MediaID:     payload.MediaID,
		VersionType: profile.VersionType,
		ObjectKey:   objectKey,
		ContentType: "video/mp4",
		Size:        int64(len(data)),
		CreatedAt:   time.Now().UTC(),
	})

	slog.Info("Video worker: profile uploaded", "profile", profile.Name, "mediaId", payload.MediaID)
	return nil
}

// generateHLS creates an HLS master playlist with all quality levels.
func (w *Worker) generateHLS(ctx context.Context, payload scanCompletedPayload, traceID, inputPath, tmpDir string) error {
	hlsDir := filepath.Join(tmpDir, "hls")
	if err := os.MkdirAll(hlsDir, 0o755); err != nil {
		return err
	}

	// Single-pass HLS output with multiple representations.
	args := []string{
		"-y", "-i", inputPath,
		// 360p stream
		"-map", "0:v", "-map", "0:a",
		"-s:v:0", "640x360", "-b:v:0", "800k", "-b:a:0", "96k",
		// 720p stream
		"-map", "0:v", "-map", "0:a",
		"-s:v:1", "1280x720", "-b:v:1", "2500k", "-b:a:1", "128k",
		// 1080p stream
		"-map", "0:v", "-map", "0:a",
		"-s:v:2", "1920x1080", "-b:v:2", "5000k", "-b:a:2", "192k",
		// Shared encoding settings
		"-c:v", "libx264", "-preset", "fast",
		"-c:a", "aac",
		// HLS output
		"-f", "hls",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments",
		"-master_pl_name", "master.m3u8",
		"-var_stream_map", "v:0,a:0 v:1,a:1 v:2,a:2",
		filepath.Join(hlsDir, "stream_%v.m3u8"),
	}

	if err := runFFmpeg(ctx, args); err != nil {
		return fmt.Errorf("ffmpeg HLS: %w", err)
	}

	// Upload all HLS segments and playlists to S3.
	baseKey := buildHLSBaseKey(payload.ObjectKey)
	entries, err := os.ReadDir(hlsDir)
	if err != nil {
		return fmt.Errorf("read HLS dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(hlsDir, entry.Name()))
		if err != nil {
			continue
		}
		ct := "video/mp2t"
		if filepath.Ext(entry.Name()) == ".m3u8" {
			ct = "application/x-mpegURL"
		}
		objKey := baseKey + "/" + entry.Name()
		if err := w.storage.PutObject(ctx, objKey, ct, data); err != nil {
			slog.Warn("Video worker: failed to upload HLS segment", "file", entry.Name(), "error", err)
		}
	}

	slog.Info("Video worker: HLS playlist uploaded", "mediaId", payload.MediaID)
	return nil
}

// runFFmpeg executes an FFmpeg command using the system-installed binary.
func runFFmpeg(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg exited with error: %w — stderr: %s", err, stderr.String())
	}
	return nil
}

// buildVersionKey derives the S3 object key for a transcoded rendition from the original key.
func buildVersionKey(originalKey, versionName, ext string) string {
	for _, suffix := range []string{"/original/", "/originals/"} {
		idx := len(originalKey) - len(originalKey)
		for i := range originalKey {
			if i+len(suffix) <= len(originalKey) && originalKey[i:i+len(suffix)] == suffix {
				idx = i
				base := originalKey[i+len(suffix):]
				// strip extension
				for j := len(base) - 1; j >= 0; j-- {
					if base[j] == '.' {
						base = base[:j]
						break
					}
				}
				return fmt.Sprintf("%s/%s/%s%s", originalKey[:idx], versionName, base, ext)
			}
		}
	}
	return originalKey + "_" + versionName + ext
}

// buildHLSBaseKey derives the base S3 prefix for HLS segments.
func buildHLSBaseKey(originalKey string) string {
	return buildVersionKey(originalKey, "hls", "")
}

// publishEvent fires a Kafka event.
func (w *Worker) publishEvent(ctx context.Context, topic, mediaID, userID, traceID string, payload map[string]interface{}) {
	data, _ := kafkaadapter.MarshalEnvelope(uuid.NewString(), topic, traceID, payload)
	if err := w.publisher.Publish(ctx, topic, mediaID, data, traceID); err != nil {
		slog.Warn("Video worker: failed to publish event", "topic", topic, "error", err)
	}
}
