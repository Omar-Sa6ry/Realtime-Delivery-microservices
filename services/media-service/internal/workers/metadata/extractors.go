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

	_ "golang.org/x/image/webp"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// extractedMeta groups the extracted values.
type extractedMeta struct {
	Width      int32
	Height     int32
	DurationMS int64
	Format     string
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

// MetadataExtractor defines the strategy interface for extracting metadata from different media types.
type MetadataExtractor interface {
	// MediaType returns the media type this extractor handles.
	MediaType() domain.MediaType

	// Extract extracts metadata from the object in storage.
	Extract(ctx context.Context, objectKey string) (extractedMeta, error)
}

// extractorRegistry holds all registered metadata extractors.
type extractorRegistry struct {
	extractors map[domain.MediaType]MetadataExtractor
}

func newExtractorRegistry() *extractorRegistry {
	return &extractorRegistry{
		extractors: make(map[domain.MediaType]MetadataExtractor),
	}
}

func (r *extractorRegistry) register(e MetadataExtractor) {
	r.extractors[e.MediaType()] = e
}

func (r *extractorRegistry) get(mediaType domain.MediaType) (MetadataExtractor, bool) {
	e, ok := r.extractors[mediaType]
	return e, ok
}

// ImageExtractor extracts metadata from image files using Go stdlib.
type ImageExtractor struct {
	storage ports.ObjectStorage
}

func NewImageExtractor(storage ports.ObjectStorage) *ImageExtractor {
	return &ImageExtractor{storage: storage}
}

func (e *ImageExtractor) MediaType() domain.MediaType {
	return domain.MediaTypeImage
}

func (e *ImageExtractor) Extract(ctx context.Context, objectKey string) (extractedMeta, error) {
	body, err := e.storage.GetObject(ctx, objectKey)
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

// VideoExtractor extracts metadata from video files using ffprobe.
type VideoExtractor struct {
	storage ports.ObjectStorage
}

func NewVideoExtractor(storage ports.ObjectStorage) *VideoExtractor {
	return &VideoExtractor{storage: storage}
}

func (e *VideoExtractor) MediaType() domain.MediaType {
	return domain.MediaTypeVideo
}

func (e *VideoExtractor) Extract(ctx context.Context, objectKey string) (extractedMeta, error) {
	// Derive a download URL — for local/test use the object key directly.
	// Production: replace with a presigned GET URL using e.storage.GeneratePresignedGET().
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

// BuildExtractorRegistry creates and populates the extractor registry.
func BuildExtractorRegistry(storage ports.ObjectStorage) *extractorRegistry {
	registry := newExtractorRegistry()
	registry.register(NewImageExtractor(storage))
	registry.register(NewVideoExtractor(storage))
	return registry
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