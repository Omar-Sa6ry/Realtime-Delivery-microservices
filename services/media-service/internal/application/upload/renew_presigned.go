package upload

import (
	"context"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// RenewPresignedInput holds parameters for renewing presigned URLs.
type RenewPresignedInput struct {
	UserID        string
	UploadID      string
	ExpirySeconds int
}

// RenewPresignedOutput holds the renewed presigned URLs.
type RenewPresignedOutput struct {
	UploadID       string
	S3UploadID     string
	PresignedParts []ports.PresignedPart
	PartSize       int64
	TotalParts     int
	ExpiresAt      time.Time
}

// RenewPresignedUseCase renews presigned URLs for an existing upload session.
type RenewPresignedUseCase struct {
	uploadRepo    ports.UploadRepository
	storage       ports.ObjectStorage
	cache         ports.Cache
	defaultExpiry time.Duration
}

// NewRenewPresignedUseCase constructs the use case.
func NewRenewPresignedUseCase(
	uploadRepo ports.UploadRepository,
	storage ports.ObjectStorage,
	cache ports.Cache,
	defaultExpiry time.Duration,
) *RenewPresignedUseCase {
	return &RenewPresignedUseCase{
		uploadRepo:    uploadRepo,
		storage:       storage,
		cache:         cache,
		defaultExpiry: defaultExpiry,
	}
}

// Execute renews the presigned URLs for an upload session.
func (uc *RenewPresignedUseCase) Execute(ctx context.Context, in RenewPresignedInput) (*RenewPresignedOutput, error) {
	// 1. Fetch upload session
	session, err := uc.uploadRepo.GetByID(ctx, in.UploadID)
	if err != nil {
		return nil, err
	}

	// 2. Authorize: ensure the user owns this session
	if session.UserID != in.UserID {
		return nil, domain.ErrUnauthorized
	}

	// 3. Check session status
	if session.Status != domain.UploadStatusUploading {
		return nil, fmt.Errorf("cannot renew presigned URLs for session in status %s", session.Status)
	}

	// 4. Check if session has expired
	if session.ExpiresAt.Before(time.Now()) {
		return nil, domain.ErrUploadExpired
	}

	// 5. Use a bounded expiry so clients cannot mint arbitrarily long-lived URLs.
	expiry := uc.defaultExpiry
	if in.ExpirySeconds > 0 && in.ExpirySeconds <= 86400 {
		expiry = time.Duration(in.ExpirySeconds) * time.Second
	}

	// 6. Generate new presigned URLs for all parts
	presignedParts, err := uc.storage.GeneratePresignedParts(
		ctx,
		session.ObjectKey,
		session.S3UploadID,
		session.TotalParts,
		session.PartSize,
		expiry,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: GeneratePresignedParts: %s", domain.ErrS3OperationFailed, err.Error())
	}

	// 7. Update the expiry and DynamoDB TTL atomically while still UPLOADING.
	newExpiry := time.Now().Add(expiry)
	if err := uc.uploadRepo.UpdateExpiry(ctx, in.UploadID, newExpiry); err != nil {
		return nil, fmt.Errorf("update session expiry: %w", err)
	}

	// 8. Update cache TTL for upload progress
	_ = uc.cache.SetUploadProgress(ctx, in.UploadID, 0, expiry)

	return &RenewPresignedOutput{
		UploadID:       session.UploadID,
		S3UploadID:     session.S3UploadID,
		PresignedParts: presignedParts,
		PartSize:       session.PartSize,
		TotalParts:     session.TotalParts,
		ExpiresAt:      newExpiry,
	}, nil
}
