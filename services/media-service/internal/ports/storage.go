package ports

import (
	"context"
	"io"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
)

// ObjectInfo contains metadata about an S3 object retrieved via HeadObject.
type ObjectInfo struct {
	Size        int64
	ETag        string
	ContentType string
	LastModified time.Time
}

// PresignedPart holds a part number and its corresponding presigned PUT URL.
type PresignedPart struct {
	PartNumber int
	PresignedURL string
}

// ObjectStorage abstracts all S3 operations.
// The service never proxies file bytes; it only manages metadata and signed URLs.
type ObjectStorage interface {
	// CreateMultipartUpload initiates a new S3 multipart upload and returns the S3 UploadId.
	CreateMultipartUpload(ctx context.Context, objectKey, contentType string) (s3UploadID string, err error)

	// GeneratePresignedParts generates N presigned PUT URLs, one per part.
	GeneratePresignedParts(ctx context.Context, objectKey, s3UploadID string, totalParts int, partSize int64, expiry time.Duration) ([]PresignedPart, error)

	// GeneratePresignedGET returns a time-limited presigned GET URL for downloading an object.
	// Presigned GET URLs are NEVER logged — they grant direct S3 access.
	GeneratePresignedGET(ctx context.Context, objectKey string, expiry time.Duration) (url string, err error)

	// CompleteMultipartUpload finalises the S3 multipart upload from the completed parts list.
	CompleteMultipartUpload(ctx context.Context, objectKey, s3UploadID string, parts []domain.UploadPart) error

	// AbortMultipartUpload cancels an in-progress S3 multipart upload and releases its parts.
	AbortMultipartUpload(ctx context.Context, objectKey, s3UploadID string) error

	// HeadObject retrieves metadata for an S3 object without downloading the body.
	HeadObject(ctx context.Context, objectKey string) (*ObjectInfo, error)

	// DeleteObject removes a single S3 object permanently.
	DeleteObject(ctx context.Context, objectKey string) error

	// DeleteObjects removes multiple S3 objects in a single batch request (up to 1000).
	DeleteObjects(ctx context.Context, objectKeys []string) error

	// GetObject streams the raw bytes of an object — used ONLY for post-upload validation
	// (magic byte check, checksum). Never called for client downloads.
	GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error)

	// PutObject uploads small objects (processed renditions < 100MB) directly without multipart.
	// Use this for resized images and short processed video clips.
	PutObject(ctx context.Context, objectKey, contentType string, data []byte) error

	// ListObjectsWithPrefix returns all object keys under a given prefix.
	// Used by the delete worker to enumerate all versions before deletion.
	ListObjectsWithPrefix(ctx context.Context, prefix string) ([]string, error)
}
