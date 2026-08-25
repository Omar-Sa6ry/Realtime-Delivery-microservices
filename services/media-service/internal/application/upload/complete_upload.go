package upload

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/validation"
)

// CompleteUploadInput holds all parameters for the CompleteUpload use case.
type CompleteUploadInput struct {
	UserID         string
	UploadID       string
	Parts          []domain.UploadPart
	IdempotencyKey string
}

// CompleteUploadOutput is returned to the gRPC handler after a successful completion.
type CompleteUploadOutput struct {
	MediaID string
	Status  domain.MediaStatus
}

// CompleteUploadUseCase orchestrates upload completion and post-upload verification.
type CompleteUploadUseCase struct {
	uploadRepo  ports.UploadRepository
	mediaRepo   ports.MediaRepository
	outboxRepo  ports.OutboxRepository
	quotaRepo   ports.QuotaRepository
	storage     ports.ObjectStorage
	cache       ports.Cache
	publisher   ports.EventPublisher
	validator   *validation.Validator
}

// NewCompleteUploadUseCase constructs the use case with all dependencies injected.
func NewCompleteUploadUseCase(
	uploadRepo ports.UploadRepository,
	mediaRepo ports.MediaRepository,
	outboxRepo ports.OutboxRepository,
	quotaRepo ports.QuotaRepository,
	storage ports.ObjectStorage,
	cache ports.Cache,
	publisher ports.EventPublisher,
	validator *validation.Validator,
) *CompleteUploadUseCase {
	return &CompleteUploadUseCase{
		uploadRepo: uploadRepo,
		mediaRepo:  mediaRepo,
		outboxRepo: outboxRepo,
		quotaRepo:  quotaRepo,
		storage:    storage,
		cache:      cache,
		publisher:  publisher,
		validator:  validator,
	}
}

// Execute runs the CompleteUpload flow:
// Fetch session → Auth → Expiry → Lock → S3 Complete → Verify → Outbox → State transition
func (uc *CompleteUploadUseCase) Execute(ctx context.Context, in CompleteUploadInput) (*CompleteUploadOutput, error) {
	logger := sharedlogging.FromContext(ctx)

	// 1. Fetch the upload session
	session, err := uc.uploadRepo.GetByID(ctx, in.UploadID)
	if err != nil {
		return nil, err
	}

	// 2. Authorization — caller must own the session
	if session.UserID != in.UserID {
		return nil, domain.ErrUnauthorized
	}

	// 3. Expiry check
	if session.IsExpired() {
		return nil, domain.ErrUploadExpired
	}

	// 4. Idempotency — allow re-calling CompleteUpload for the same session
	if in.IdempotencyKey != "" {
		idem, err := uc.cache.CheckAndStoreIdempotency(ctx, in.UserID, "complete:"+in.UploadID, nil, 24*time.Hour)
		if err == nil && idem.Exists {
			// Already completed — return success
			media, _ := uc.mediaRepo.GetByID(ctx, session.MediaID)
			if media != nil {
				return &CompleteUploadOutput{MediaID: media.MediaID, Status: media.Status}, nil
			}
		}
	}

	// 5. Acquire distributed lock — prevent concurrent completion attempts
	lockToken, acquired, err := uc.cache.AcquireLock(ctx, session.MediaID, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("acquire completion lock: %w", err)
	}
	if !acquired {
		return nil, fmt.Errorf("upload completion already in progress for media %q", session.MediaID)
	}
	defer func() {
		if releaseErr := uc.cache.ReleaseLock(ctx, session.MediaID, lockToken); releaseErr != nil {
			logger.Error("Failed to release completion lock", "mediaId", session.MediaID, "error", releaseErr)
		}
	}()

	// 6. Complete S3 multipart upload
	if err := uc.storage.CompleteMultipartUpload(ctx, session.ObjectKey, session.S3UploadID, in.Parts); err != nil {
		return nil, fmt.Errorf("%w: S3 CompleteMultipartUpload: %s", domain.ErrS3OperationFailed, err.Error())
	}

	// 7. Verify the assembled object in S3
	objInfo, err := uc.storage.HeadObject(ctx, session.ObjectKey)
	if err != nil {
		return nil, fmt.Errorf("%w: HeadObject after completion: %s", domain.ErrS3OperationFailed, err.Error())
	}

	// 8. Fetch media to get the declared size and checksum
	media, err := uc.mediaRepo.GetByID(ctx, session.MediaID)
	if err != nil {
		return nil, fmt.Errorf("fetch media after completion: %w", err)
	}

	// 9. Size integrity check
	if objInfo.Size != media.Size {
		return nil, fmt.Errorf("%w: declared size %d, actual S3 size %d", domain.ErrChecksumMismatch, media.Size, objInfo.Size)
	}

	// 10. Post-upload validation (magic bytes + optional checksum)
	if err := uc.validator.ValidatePostUpload(ctx, session.ObjectKey, media.ContentType, media.Checksum); err != nil {
		// Mark as failed and abort
		_ = uc.mediaRepo.UpdateStatus(ctx, session.MediaID, domain.MediaStatusUploading, domain.MediaStatusFailed)
		return nil, err
	}

	// 11. Build and persist the outbox event BEFORE updating media status.
	// This ensures the event is not lost if the process crashes between steps.
	eventPayload, _ := kafka.MarshalEnvelope(
		uuid.NewString(),
		kafka.TopicUploadCompleted,
		sharedlogging.GetTraceID(ctx),
		map[string]interface{}{
			"mediaId":     media.MediaID,
			"userId":      in.UserID,
			"fileName":    media.FileName,
			"objectKey":   session.ObjectKey,
			"size":        media.Size,
			"contentType": media.ContentType,
			"mediaType":   string(media.MediaType),
		},
	)
	outboxEvent := &domain.OutboxEvent{
		EventID:     uuid.NewString(),
		AggregateID: media.MediaID,
		EventType:   kafka.TopicUploadCompleted,
		Payload:     eventPayload,
		Status:      domain.OutboxStatusPending,
		TraceID:     sharedlogging.GetTraceID(ctx),
		CreatedAt:   time.Now().UTC(),
		TTL:         time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	if err := uc.outboxRepo.Create(ctx, outboxEvent); err != nil {
		logger.Error("Failed to create outbox event", "mediaId", media.MediaID, "error", err)
		// Non-fatal — outbox publisher will retry
	}

	// 12. Transition media state UPLOADING -> UPLOADED (conditional update)
	if err := uc.mediaRepo.UpdateStatus(ctx, session.MediaID, domain.MediaStatusUploading, domain.MediaStatusUploaded); err != nil {
		return nil, fmt.Errorf("media state transition UPLOADING->UPLOADED: %w", err)
	}

	// 13. Update upload session status
	_ = uc.uploadRepo.UpdateStatus(ctx, in.UploadID, domain.UploadStatusCompleted)

	// 14. Decrement active upload counter and commit used bytes
	_ = uc.quotaRepo.DecrementActiveUpload(ctx, in.UserID)
	_ = uc.quotaRepo.AddUsedBytes(ctx, in.UserID, media.Size)

	logger.Info("Upload completed successfully",
		"mediaId", media.MediaID,
		"uploadId", in.UploadID,
		"userId", in.UserID,
		"size", media.Size,
	)

	return &CompleteUploadOutput{MediaID: media.MediaID, Status: domain.MediaStatusUploaded}, nil
}
