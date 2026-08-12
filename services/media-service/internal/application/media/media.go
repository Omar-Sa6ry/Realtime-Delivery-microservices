package media

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// GetMediaUseCase retrieves a single media item by ID.
type GetMediaUseCase struct {
	mediaRepo   ports.MediaRepository
	versionRepo ports.VersionRepository
}

// NewGetMediaUseCase constructs the use case.
func NewGetMediaUseCase(mediaRepo ports.MediaRepository, versionRepo ports.VersionRepository) *GetMediaUseCase {
	return &GetMediaUseCase{mediaRepo: mediaRepo, versionRepo: versionRepo}
}

// MediaOutput combines a media item with its processed versions.
type MediaOutput struct {
	Media    *domain.Media
	Versions []*domain.MediaVersion
}

// Execute fetches the media and its versions, enforcing ownership.
func (uc *GetMediaUseCase) Execute(ctx context.Context, userID, mediaID string) (*MediaOutput, error) {
	m, err := uc.mediaRepo.GetByID(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	if m.OwnerID != userID {
		return nil, domain.ErrUnauthorized
	}

	versions, err := uc.versionRepo.ListByMedia(ctx, mediaID)
	if err != nil {
		versions = []*domain.MediaVersion{} // non-fatal — return empty versions
	}

	return &MediaOutput{Media: m, Versions: versions}, nil
}

// ListMediaInput holds parameters for listing media items.
type ListMediaInput struct {
	OwnerID      string
	Limit        int
	Cursor       string
	StatusFilter string
}

// ListMediaUseCase lists paginated media items for an owner.
type ListMediaUseCase struct {
	mediaRepo ports.MediaRepository
}

// NewListMediaUseCase constructs the use case.
func NewListMediaUseCase(mediaRepo ports.MediaRepository) *ListMediaUseCase {
	return &ListMediaUseCase{mediaRepo: mediaRepo}
}

// Execute returns a paginated list of media items.
func (uc *ListMediaUseCase) Execute(ctx context.Context, in ListMediaInput) ([]*domain.Media, string, error) {
	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return uc.mediaRepo.ListByOwner(ctx, in.OwnerID, limit, in.Cursor)
}

// DeleteMediaUseCase handles the asynchronous deletion of a media item.
type DeleteMediaUseCase struct {
	mediaRepo  ports.MediaRepository
	outboxRepo ports.OutboxRepository
	quotaRepo  ports.QuotaRepository
	cache      ports.Cache
}

// NewDeleteMediaUseCase constructs the use case.
func NewDeleteMediaUseCase(
	mediaRepo ports.MediaRepository,
	outboxRepo ports.OutboxRepository,
	quotaRepo ports.QuotaRepository,
	cache ports.Cache,
) *DeleteMediaUseCase {
	return &DeleteMediaUseCase{
		mediaRepo:  mediaRepo,
		outboxRepo: outboxRepo,
		quotaRepo:  quotaRepo,
		cache:      cache,
	}
}

// Execute marks a media item for deletion and enqueues a delete event via the outbox.
// Actual S3 deletion is handled asynchronously by the delete worker.
func (uc *DeleteMediaUseCase) Execute(ctx context.Context, userID, mediaID, idempotencyKey string) error {
	logger := sharedlogging.FromContext(ctx)

	// 1. Fetch and authorise
	m, err := uc.mediaRepo.GetByID(ctx, mediaID)
	if err != nil {
		return err
	}
	if m.OwnerID != userID {
		return domain.ErrUnauthorized
	}

	// 2. Idempotency
	if idempotencyKey != "" {
		idem, _ := uc.cache.CheckAndStoreIdempotency(ctx, userID, "delete:"+mediaID, nil, 24*time.Hour)
		if idem != nil && idem.Exists {
			return nil // already accepted
		}
	}

	// 3. Transition to DELETING (state machine validates the transition)
	if !m.CanTransitionTo(domain.MediaStatusDeleting) {
		return fmt.Errorf("%w: cannot delete media in state %q", domain.ErrInvalidStateTransition, m.Status)
	}
	if err := uc.mediaRepo.UpdateStatus(ctx, mediaID, m.Status, domain.MediaStatusDeleting); err != nil {
		return fmt.Errorf("transition to DELETING: %w", err)
	}

	// 4. Create outbox event for the delete worker
	eventPayload, _ := kafka.MarshalEnvelope(
		uuid.NewString(), kafka.TopicDeleteRequested, sharedlogging.GetTraceID(ctx),
		map[string]interface{}{
			"mediaId":   mediaID,
			"userId":    userID,
			"objectKey": m.ObjectKey,
		},
	)
	outboxEvent := &domain.OutboxEvent{
		EventID:     uuid.NewString(),
		AggregateID: mediaID,
		EventType:   kafka.TopicDeleteRequested,
		Payload:     eventPayload,
		Status:      domain.OutboxStatusPending,
		TraceID:     sharedlogging.GetTraceID(ctx),
		CreatedAt:   time.Now().UTC(),
		TTL:         time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	if err := uc.outboxRepo.Create(ctx, outboxEvent); err != nil {
		logger.Error("Failed to create delete outbox event", "mediaId", mediaID, "error", err)
	}

	logger.Info("Media deletion accepted", "mediaId", mediaID, "userId", userID)
	return nil
}
