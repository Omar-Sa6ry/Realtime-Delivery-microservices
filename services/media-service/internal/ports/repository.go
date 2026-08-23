package ports

import (
	"context"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
)

// MediaRepository abstracts all DynamoDB operations for the Media aggregate.
type MediaRepository interface {
	// Create inserts a new Media record. Returns an error if it already exists.
	Create(ctx context.Context, media *domain.Media) error

	// GetByID retrieves a Media record by its primary key.
	GetByID(ctx context.Context, mediaID string) (*domain.Media, error)

	// UpdateStatus performs a conditional state-machine transition in DynamoDB.
	// The update only proceeds if the current status equals expectedCurrent.
	// This prevents concurrent workers from stomping each other's state.
	UpdateStatus(ctx context.Context, mediaID string, expectedCurrent, next domain.MediaStatus) error

	// ListByOwner returns paginated media records for a given owner.
	// cursor is the last evaluated DynamoDB key (empty string for first page).
	ListByOwner(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Media, string, error)

	// Delete marks a media record as DELETED. Physical deletion is handled by the delete worker.
	Delete(ctx context.Context, mediaID string) error
}

// UploadRepository abstracts all DynamoDB operations for UploadSession.
type UploadRepository interface {
	// Create inserts a new UploadSession record.
	Create(ctx context.Context, session *domain.UploadSession) error

	// GetByID retrieves an UploadSession by its primary key.
	GetByID(ctx context.Context, uploadID string) (*domain.UploadSession, error)

	// UpdateStatus updates the status of an upload session.
	UpdateStatus(ctx context.Context, uploadID string, status domain.UploadStatus) error

	// UpdateCompletedParts atomically appends completed parts to the session record.
	UpdateCompletedParts(ctx context.Context, uploadID string, parts []domain.UploadPart) error

	// UpdateExpiry atomically extends the upload session expiry and TTL.
	UpdateExpiry(ctx context.Context, uploadID string, expiresAt time.Time) error

	// ListExpired returns upload sessions that expired before the given time.
	// Used by the reconciliation worker.
	ListExpired(ctx context.Context, before time.Time) ([]*domain.UploadSession, error)
}

// JobRepository abstracts all DynamoDB operations for MediaJob.
type JobRepository interface {
	// Create inserts a new processing job.
	Create(ctx context.Context, job *domain.MediaJob) error

	// GetByID retrieves a job by its primary key.
	GetByID(ctx context.Context, jobID string) (*domain.MediaJob, error)

	// UpdateStatus updates the job status and increments the attempt counter.
	UpdateStatus(ctx context.Context, jobID string, status domain.JobStatus, lastError string) error

	// ListStuck returns jobs in RUNNING state that started before the given time.
	// Used by the reconciliation worker to detect and recover stuck jobs.
	ListStuck(ctx context.Context, stuckBefore time.Time) ([]*domain.MediaJob, error)
}

// OutboxRepository abstracts all DynamoDB operations for OutboxEvent.
type OutboxRepository interface {
	// CreateInTransaction writes an outbox event as part of a DynamoDB transaction.
	// The tx parameter is a DynamoDB TransactWriteItems that the caller builds.
	Create(ctx context.Context, event *domain.OutboxEvent) error

	// ListPending returns unpublished outbox events, ordered by creation time.
	ListPending(ctx context.Context, limit int) ([]*domain.OutboxEvent, error)

	// MarkPublished atomically marks an event as PUBLISHED.
	MarkPublished(ctx context.Context, eventID string) error

	// MarkFailed marks an event as FAILED after exhausting retries.
	MarkFailed(ctx context.Context, eventID string, reason string) error
}

// QuotaRepository abstracts all DynamoDB atomic counter operations for user quota.
type QuotaRepository interface {
	// GetUsage retrieves the current storage quota usage for a user.
	GetUsage(ctx context.Context, userID string) (*domain.QuotaUsage, error)

	// IncrementUsage atomically adds bytes and increments the active upload counter.
	// Called when an upload session is created (reserving quota).
	IncrementUsage(ctx context.Context, userID string, bytes int64) error

	// DecrementUsage atomically releases bytes and decrements the active upload counter.
	// Called when an upload completes, is aborted, or fails.
	DecrementActiveUpload(ctx context.Context, userID string) error

	// AddUsedBytes atomically adds bytes to the committed usage.
	// Called when an upload is fully completed and verified.
	AddUsedBytes(ctx context.Context, userID string, bytes int64) error

	// SubtractUsedBytes atomically removes bytes from the committed usage.
	// Called when a media item is deleted.
	SubtractUsedBytes(ctx context.Context, userID string, bytes int64) error
}

// VersionRepository abstracts DynamoDB operations for MediaVersion.
type VersionRepository interface {
	// Create inserts a new MediaVersion record.
	Create(ctx context.Context, version *domain.MediaVersion) error

	// ListByMedia returns all processed versions for a given media item.
	ListByMedia(ctx context.Context, mediaID string) ([]*domain.MediaVersion, error)
}
