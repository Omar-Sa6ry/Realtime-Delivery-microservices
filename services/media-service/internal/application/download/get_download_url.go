package download

import (
	"context"
	"fmt"
	"time"

	sharedratelimiter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/ratelimiter"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// GetDownloadUrlInput holds parameters for the download URL generation.
type GetDownloadUrlInput struct {
	UserID      string
	MediaID     string
	VersionType string // "original", "thumbnail", "720p", etc. — empty = original
	ExpirySeconds int
}

// GetDownloadUrlOutput holds the presigned download URL and metadata.
// SECURITY: This URL is returned to the caller but NEVER logged.
type GetDownloadUrlOutput struct {
	URL         string
	ExpiresAt   time.Time
	ContentType string
}

// GetDownloadUrlUseCase generates a presigned GET URL for authorised media access.
type GetDownloadUrlUseCase struct {
	mediaRepo    ports.MediaRepository
	versionRepo  ports.VersionRepository
	storage      ports.ObjectStorage
	rateLimiter  *sharedratelimiter.RateLimiter
	defaultExpiry time.Duration
}

// NewGetDownloadUrlUseCase constructs the use case.
func NewGetDownloadUrlUseCase(
	mediaRepo ports.MediaRepository,
	versionRepo ports.VersionRepository,
	storage ports.ObjectStorage,
	rateLimiter *sharedratelimiter.RateLimiter,
	defaultExpiry time.Duration,
) *GetDownloadUrlUseCase {
	return &GetDownloadUrlUseCase{
		mediaRepo:    mediaRepo,
		versionRepo:  versionRepo,
		storage:      storage,
		rateLimiter:  rateLimiter,
		defaultExpiry: defaultExpiry,
	}
}

// Execute performs auth → rate limit → presign URL generation.
// The presigned URL is returned directly to the client; S3 handles the download.
// This service NEVER proxies file bytes — download traffic bypasses the application entirely.
func (uc *GetDownloadUrlUseCase) Execute(ctx context.Context, in GetDownloadUrlInput) (*GetDownloadUrlOutput, error) {
	// 1. Rate limit download URL generation
	rateLimitKey := fmt.Sprintf("download:%s", in.UserID)
	result, err := uc.rateLimiter.Limit(ctx, rateLimitKey)
	if err != nil {
		return nil, fmt.Errorf("rate limiter check: %w", err)
	}
	if !result.Allowed {
		return nil, domain.ErrRateLimitExceeded
	}

	// 2. Fetch and authorise
	m, err := uc.mediaRepo.GetByID(ctx, in.MediaID)
	if err != nil {
		return nil, err
	}
	if m.OwnerID != in.UserID {
		return nil, domain.ErrUnauthorized
	}
	if m.Status == domain.MediaStatusQuarantined {
		return nil, domain.ErrMediaQuarantined
	}
	if m.Status == domain.MediaStatusFailed || m.Status == domain.MediaStatusDeleted || m.Status == domain.MediaStatusAborted {
		return nil, fmt.Errorf("media is not available for download (status: %s)", m.Status)
	}

	// 3. Determine the object key based on the requested version
	objectKey := m.ObjectKey
	contentType := m.ContentType

	if in.VersionType != "" && in.VersionType != "original" {
		versions, err := uc.versionRepo.ListByMedia(ctx, in.MediaID)
		if err == nil {
			for _, v := range versions {
				if string(v.VersionType) == in.VersionType {
					objectKey = v.ObjectKey
					contentType = v.ContentType
					break
				}
			}
		}
	}

	// 4. Determine expiry
	expiry := uc.defaultExpiry
	if in.ExpirySeconds > 0 && in.ExpirySeconds <= 86400 {
		expiry = time.Duration(in.ExpirySeconds) * time.Second
	}

	// 5. Generate presigned GET URL
	// SECURITY: URL is NOT logged — it grants direct S3 access.
	url, err := uc.storage.GeneratePresignedGET(ctx, objectKey, expiry)
	if err != nil {
		return nil, fmt.Errorf("%w: GeneratePresignedGET: %s", domain.ErrS3OperationFailed, err.Error())
	}

	return &GetDownloadUrlOutput{
		URL:         url,
		ExpiresAt:   time.Now().Add(expiry),
		ContentType: contentType,
	}, nil
}
