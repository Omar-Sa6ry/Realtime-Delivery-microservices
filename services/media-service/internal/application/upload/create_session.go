package upload

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	sharedratelimiter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/ratelimiter"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/validation"
)

// CreateSessionInput holds all parameters for the CreateUploadSession use case.
type CreateSessionInput struct {
	UserID         string
	FileName       string
	ContentType    string
	Size           int64
	Checksum       string // optional client SHA-256
	DeliveryID     string // optional delivery context
	IdempotencyKey string
}

// CreateSessionOutput is the result returned to the gRPC handler.
type CreateSessionOutput struct {
	MediaID       string
	UploadID      string
	S3UploadID    string
	PresignedParts []ports.PresignedPart
	PartSize      int64
	TotalParts    int
	ExpiresAt     time.Time
}

// CreateSessionUseCase orchestrates the upload session creation flow.
type CreateSessionUseCase struct {
	mediaRepo  ports.MediaRepository
	uploadRepo ports.UploadRepository
	quotaRepo  ports.QuotaRepository
	storage    ports.ObjectStorage
	cache      ports.Cache
	validator  *validation.Validator
	rateLimiter *sharedratelimiter.RateLimiter
	// Config values
	sessionTTL     time.Duration
	presignExpiry  time.Duration
	minPartSize    int64
	maxFileSize    int64
}

// NewCreateSessionUseCase constructs the use case with all dependencies injected.
func NewCreateSessionUseCase(
	mediaRepo ports.MediaRepository,
	uploadRepo ports.UploadRepository,
	quotaRepo ports.QuotaRepository,
	storage ports.ObjectStorage,
	cache ports.Cache,
	validator *validation.Validator,
	rateLimiter *sharedratelimiter.RateLimiter,
	sessionTTL, presignExpiry time.Duration,
	minPartSize, maxFileSize int64,
) *CreateSessionUseCase {
	return &CreateSessionUseCase{
		mediaRepo:   mediaRepo,
		uploadRepo:  uploadRepo,
		quotaRepo:   quotaRepo,
		storage:     storage,
		cache:       cache,
		validator:   validator,
		rateLimiter: rateLimiter,
		sessionTTL:  sessionTTL,
		presignExpiry: presignExpiry,
		minPartSize: minPartSize,
		maxFileSize: maxFileSize,
	}
}

// Execute runs the full CreateUploadSession flow:
// Auth → Validate → Quota → RateLimit → Idempotency → Create S3 Multipart → Persist → Return
func (uc *CreateSessionUseCase) Execute(ctx context.Context, in CreateSessionInput) (*CreateSessionOutput, error) {
	logger := sharedlogging.FromContext(ctx)

	// 1. Input validation (size, content type, extension)
	if err := uc.validator.ValidateUploadRequest(validation.UploadRequest{
		FileName:    in.FileName,
		ContentType: in.ContentType,
		Size:        in.Size,
		Checksum:    in.Checksum,
	}); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	// 2. Rate limit check (uses shared Redis sliding window from packages/go/ratelimiter)
	rateLimitKey := fmt.Sprintf("upload:%s", in.UserID)
	result, err := uc.rateLimiter.Limit(ctx, rateLimitKey)
	if err != nil {
		return nil, fmt.Errorf("rate limiter check: %w", err)
	}
	if !result.Allowed {
		return nil, domain.ErrRateLimitExceeded
	}

	// 3. Idempotency check — prevent duplicate sessions for the same request
	if in.IdempotencyKey != "" {
		idem, err := uc.cache.CheckAndStoreIdempotency(ctx, in.UserID, in.IdempotencyKey, nil, 24*time.Hour)
		if err != nil {
			return nil, fmt.Errorf("idempotency check: %w", err)
		}
		if idem.Exists {
			return nil, domain.ErrIdempotencyConflict
		}
	}

	// 4. Quota check (DynamoDB read)
	quota, err := uc.quotaRepo.GetUsage(ctx, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("quota read: %w", err)
	}
	if !quota.CanUpload(in.Size) {
		return nil, domain.ErrQuotaExceeded
	}
	if !quota.HasUploadSlot() {
		return nil, domain.ErrConcurrentUploadsExceeded
	}

	// 5. Generate IDs and object key
	mediaID := uuid.NewString()
	uploadID := uuid.NewString()
	normalizedName := validation.NormalizeFileName(in.FileName)
	objectKey := buildObjectKey(in.UserID, in.DeliveryID, mediaID, normalizedName)

	// 6. Calculate multipart parameters
	totalParts, partSize := calculateParts(in.Size, uc.minPartSize)

	// 7. Create S3 multipart upload
	s3UploadID, err := uc.storage.CreateMultipartUpload(ctx, objectKey, in.ContentType)
	if err != nil {
		return nil, fmt.Errorf("%w: CreateMultipartUpload: %s", domain.ErrS3OperationFailed, err.Error())
	}

	// 8. Generate presigned PUT URLs for each part
	presignedParts, err := uc.storage.GeneratePresignedParts(ctx, objectKey, s3UploadID, totalParts, partSize, uc.presignExpiry)
	if err != nil {
		// Abort the multipart upload if URL generation fails — prevent orphaned uploads.
		_ = uc.storage.AbortMultipartUpload(ctx, objectKey, s3UploadID)
		return nil, fmt.Errorf("%w: GeneratePresignedParts: %s", domain.ErrS3OperationFailed, err.Error())
	}

	// 9. Persist Media record (PENDING)
	now := time.Now().UTC()
	media := &domain.Media{
		MediaID:     mediaID,
		OwnerID:     in.UserID,
		DeliveryID:  in.DeliveryID,
		FileName:    normalizedName,
		ContentType: in.ContentType,
		MediaType:   domain.DeriveMediaType(in.ContentType),
		Size:        in.Size,
		Checksum:    in.Checksum,
		ObjectKey:   objectKey,
		Status:      domain.MediaStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := uc.mediaRepo.Create(ctx, media); err != nil {
		_ = uc.storage.AbortMultipartUpload(ctx, objectKey, s3UploadID)
		return nil, fmt.Errorf("persist media record: %w", err)
	}

	// 10. Persist UploadSession (UPLOADING)
	expiresAt := now.Add(uc.sessionTTL)
	session := &domain.UploadSession{
		UploadID:       uploadID,
		MediaID:        mediaID,
		UserID:         in.UserID,
		S3UploadID:     s3UploadID,
		ObjectKey:      objectKey,
		TotalParts:     totalParts,
		CompletedParts: nil,
		Status:         domain.UploadStatusUploading,
		ExpiresAt:      expiresAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := uc.uploadRepo.Create(ctx, session); err != nil {
		_ = uc.storage.AbortMultipartUpload(ctx, objectKey, s3UploadID)
		return nil, fmt.Errorf("persist upload session: %w", err)
	}

	// 11. Transition Media from PENDING -> UPLOADING
	if err := uc.mediaRepo.UpdateStatus(ctx, mediaID, domain.MediaStatusPending, domain.MediaStatusUploading); err != nil {
		logger.Warn("Failed to transition media to UPLOADING", "mediaId", mediaID, "error", err)
	}

	// 12. Increment active upload counter (non-blocking)
	if err := uc.quotaRepo.IncrementUsage(ctx, in.UserID, in.Size); err != nil {
		logger.Error("Failed to increment active upload counter", "userId", in.UserID, "error", err)
	}

	logger.Info("Upload session created",
		"mediaId", mediaID,
		"uploadId", uploadID,
		"userId", in.UserID,
		"size", in.Size,
		"totalParts", totalParts,
	)

	return &CreateSessionOutput{
		MediaID:        mediaID,
		UploadID:       uploadID,
		S3UploadID:     s3UploadID,
		PresignedParts: presignedParts,
		PartSize:       partSize,
		TotalParts:     totalParts,
		ExpiresAt:      expiresAt,
	}, nil
}

// buildObjectKey constructs the S3 object key in a structured, secure format.
// The client never controls the key — only the server generates it.
func buildObjectKey(userID, deliveryID, mediaID, fileName string) string {
	if deliveryID != "" {
		return fmt.Sprintf("users/%s/deliveries/%s/media/%s/original/%s", userID, deliveryID, mediaID, fileName)
	}
	return fmt.Sprintf("users/%s/media/%s/original/%s", userID, mediaID, fileName)
}

// calculateParts determines the optimal number of parts and part size for a file.
// S3 requires a minimum part size of 5 MiB (except for the last part).
// S3 allows a maximum of 10,000 parts.
func calculateParts(fileSize, minPartSize int64) (totalParts int, partSize int64) {
	if fileSize <= minPartSize {
		return 1, fileSize
	}
	parts := int(math.Ceil(float64(fileSize) / float64(minPartSize)))
	if parts > 10000 {
		parts = 10000
		partSize = int64(math.Ceil(float64(fileSize) / float64(parts)))
	} else {
		partSize = minPartSize
	}
	return parts, partSize
}
