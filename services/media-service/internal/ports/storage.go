package ports

import (
	"context"
	"io"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
)

type ObjectInfo struct {
	Size        int64
	ETag        string
	ContentType string
	LastModified time.Time
}

type PresignedPart struct {
	PartNumber int
	PresignedURL string
}

type ObjectStorage interface {
	CreateMultipartUpload(ctx context.Context, objectKey, contentType string) (s3UploadID string, err error)
	GeneratePresignedParts(ctx context.Context, objectKey, s3UploadID string, totalParts int, partSize int64, expiry time.Duration) ([]PresignedPart, error)
	GeneratePresignedGET(ctx context.Context, objectKey string, expiry time.Duration) (url string, err error)
	GeneratePresignedGETWithRange(ctx context.Context, objectKey, rangeHeader string, expiry time.Duration) (url string, err error)
	CompleteMultipartUpload(ctx context.Context, objectKey, s3UploadID string, parts []domain.UploadPart) error
	AbortMultipartUpload(ctx context.Context, objectKey, s3UploadID string) error
	HeadObject(ctx context.Context, objectKey string) (*ObjectInfo, error)
	DeleteObject(ctx context.Context, objectKey string) error
	DeleteObjects(ctx context.Context, objectKeys []string) error
	GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error)
	GetObjectWithRange(ctx context.Context, objectKey, rangeHeader string) (io.ReadCloser, error)
	PutObject(ctx context.Context, objectKey, contentType string, data []byte) error
	ListObjectsWithPrefix(ctx context.Context, prefix string) ([]string, error)
}
