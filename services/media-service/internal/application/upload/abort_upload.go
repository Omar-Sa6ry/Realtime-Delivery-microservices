package upload

import (
	"context"
	"fmt"
	"time"

	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/observability"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

type AbortUploadUseCase struct {
	uploadRepo ports.UploadRepository
	mediaRepo  ports.MediaRepository
	quotaRepo  ports.QuotaRepository
	storage    ports.ObjectStorage
	cache      ports.Cache
}

func NewAbortUploadUseCase(
	uploadRepo ports.UploadRepository,
	mediaRepo ports.MediaRepository,
	quotaRepo ports.QuotaRepository,
	storage ports.ObjectStorage,
	cache ports.Cache,
) *AbortUploadUseCase {
	return &AbortUploadUseCase{
		uploadRepo: uploadRepo,
		mediaRepo:  mediaRepo,
		quotaRepo:  quotaRepo,
		storage:    storage,
		cache:      cache,
	}
}

// Fetch → Auth → Lock → S3 Abort → State transition → Quota release
func (uc *AbortUploadUseCase) Execute(ctx context.Context, userID, uploadID string) error {
	logger := sharedlogging.FromContext(ctx)

	session, err := uc.uploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		return err
	}

	if session.UserID != userID {
		return domain.ErrUnauthorized
	}

	// Acquire lock to prevent concurrent abort + complete race
	lockToken, acquired, err := uc.cache.AcquireLock(ctx, session.MediaID, 15*time.Second)
	if err != nil {
		return fmt.Errorf("acquire abort lock: %w", err)
	}
	if !acquired {
		return fmt.Errorf("media %q is currently being processed", session.MediaID)
	}
	defer uc.cache.ReleaseLock(ctx, session.MediaID, lockToken)

	// Abort the S3 multipart upload — frees parts storage
	if err := uc.storage.AbortMultipartUpload(ctx, session.ObjectKey, session.S3UploadID); err != nil {
		logger.Error("Failed to abort S3 multipart upload", "objectKey", session.ObjectKey, "error", err)
		// Continue — the reconciliation worker and S3 lifecycle policy will clean up
	}

	// Mark upload session as ABORTED
	if err := uc.uploadRepo.UpdateStatus(ctx, uploadID, domain.UploadStatusAborted); err != nil {
		logger.Error("Failed to mark upload session aborted", "uploadId", uploadID, "error", err)
	}

	// Transition media to ABORTED
	if err := uc.mediaRepo.UpdateStatus(ctx, session.MediaID, domain.MediaStatusUploading, domain.MediaStatusAborted); err != nil {
		logger.Error("Failed to transition media to ABORTED", "mediaId", session.MediaID, "error", err)
	}

	// Release the active upload slot
	_ = uc.quotaRepo.DecrementActiveUpload(ctx, userID)
	observability.DecActiveUploads()

	logger.Info("Upload aborted", "uploadId", uploadID, "mediaId", session.MediaID, "userId", userID)
	return nil
}

type GetUploadStatusUseCase struct {
	uploadRepo ports.UploadRepository
	storage    ports.ObjectStorage
}

func NewGetUploadStatusUseCase(uploadRepo ports.UploadRepository, storage ports.ObjectStorage) *GetUploadStatusUseCase {
	return &GetUploadStatusUseCase{uploadRepo: uploadRepo, storage: storage}
}

type UploadStatusOutput struct {
	UploadID       string
	Status         domain.UploadStatus
	TotalParts     int
	CompletedParts int
	MissingParts   []int
	ExpiresAt      time.Time
}

func (uc *GetUploadStatusUseCase) Execute(ctx context.Context, userID, uploadID string) (*UploadStatusOutput, error) {
	session, err := uc.uploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, domain.ErrUnauthorized
	}

	return &UploadStatusOutput{
		UploadID:       session.UploadID,
		Status:         session.Status,
		TotalParts:     session.TotalParts,
		CompletedParts: len(session.CompletedParts),
		MissingParts:   session.MissingPartNumbers(),
		ExpiresAt:      session.ExpiresAt,
	}, nil
}
