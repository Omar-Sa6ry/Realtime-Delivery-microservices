package ports

import (
	"context"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
)

type MediaRepository interface {
	Create(ctx context.Context, media *domain.Media) error
	GetByID(ctx context.Context, mediaID string) (*domain.Media, error)
	UpdateStatus(ctx context.Context, mediaID string, expectedCurrent, next domain.MediaStatus) error
	ListByOwner(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Media, string, error)
	Delete(ctx context.Context, mediaID string) error
}

type UploadRepository interface {
	Create(ctx context.Context, session *domain.UploadSession) error
	GetByID(ctx context.Context, uploadID string) (*domain.UploadSession, error)
	UpdateStatus(ctx context.Context, uploadID string, status domain.UploadStatus) error
	UpdateCompletedParts(ctx context.Context, uploadID string, parts []domain.UploadPart) error
	UpdateExpiry(ctx context.Context, uploadID string, expiresAt time.Time) error
	ListExpired(ctx context.Context, before time.Time) ([]*domain.UploadSession, error)
}

type JobRepository interface {
	Create(ctx context.Context, job *domain.MediaJob) error
	GetByID(ctx context.Context, jobID string) (*domain.MediaJob, error)
	UpdateStatus(ctx context.Context, jobID string, status domain.JobStatus, lastError string) error
	ListStuck(ctx context.Context, stuckBefore time.Time) ([]*domain.MediaJob, error)
}

type OutboxRepository interface {
	Create(ctx context.Context, event *domain.OutboxEvent) error
	ListPending(ctx context.Context, limit int) ([]*domain.OutboxEvent, error)
	MarkPublished(ctx context.Context, eventID string) error
	MarkFailed(ctx context.Context, eventID string, reason string) error
}

type QuotaRepository interface {
	GetUsage(ctx context.Context, userID string) (*domain.QuotaUsage, error)
	IncrementUsage(ctx context.Context, userID string, bytes int64) error
	DecrementActiveUpload(ctx context.Context, userID string) error
	AddUsedBytes(ctx context.Context, userID string, bytes int64) error
	SubtractUsedBytes(ctx context.Context, userID string, bytes int64) error
}

type VersionRepository interface {
	Create(ctx context.Context, version *domain.MediaVersion) error
	ListByMedia(ctx context.Context, mediaID string) ([]*domain.MediaVersion, error)
}
