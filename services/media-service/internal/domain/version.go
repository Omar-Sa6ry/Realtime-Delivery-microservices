package domain

import "time"

// VersionType represents a specific processed rendition of a media item.
type VersionType string

const (
	VersionTypeOriginal   VersionType = "original"
	VersionTypeThumbnail  VersionType = "thumbnail"
	VersionTypeMedium     VersionType = "medium"
	VersionTypeOptimized  VersionType = "optimized"
	VersionType360p       VersionType = "360p"
	VersionType720p       VersionType = "720p"
	VersionType1080p      VersionType = "1080p"
	VersionTypeWebP       VersionType = "webp"
	VersionTypeAVIF       VersionType = "avif"
	VersionTypePreview    VersionType = "preview"
	VersionTypeCompressed VersionType = "compressed" // gzip-compressed rendition
	VersionTypeHLS        VersionType = "hls"        // HLS adaptive streaming master playlist
)

type MediaVersion struct {
	VersionID   string      `json:"versionId"`
	MediaID     string      `json:"mediaId"`
	VersionType VersionType `json:"versionType"`
	ObjectKey   string      `json:"objectKey"`
	ContentType string      `json:"contentType"`
	Size        int64       `json:"size"`
	Checksum    string      `json:"checksum"`
	Width       int32       `json:"width,omitempty"`
	Height      int32       `json:"height,omitempty"`
	DurationMS  int64       `json:"durationMs,omitempty"`
	CreatedAt   time.Time   `json:"createdAt"`
}
