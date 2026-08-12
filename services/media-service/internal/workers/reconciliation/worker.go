package reconciliation

import (
	"context"
	"log/slog"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// Worker is the reconciliation worker that detects and repairs stuck operations.
// It runs on a schedule and is safe to run on multiple replicas (distributed lock prevents double-run).
type Worker struct {
	uploadRepo             ports.UploadRepository
	mediaRepo              ports.MediaRepository
	outboxRepo             ports.OutboxRepository
	storage                ports.ObjectStorage
	cache                  ports.Cache
	publisher              ports.EventPublisher
	stuckUploadTimeout     time.Duration
	stuckProcessingTimeout time.Duration
}

// NewWorker creates a new reconciliation worker.
func NewWorker(
	uploadRepo ports.UploadRepository,
	mediaRepo ports.MediaRepository,
	outboxRepo ports.OutboxRepository,
	storage ports.ObjectStorage,
	cache ports.Cache,
	publisher ports.EventPublisher,
	stuckUploadTimeout, stuckProcessingTimeout time.Duration,
) *Worker {
	return &Worker{
		uploadRepo:             uploadRepo,
		mediaRepo:              mediaRepo,
		outboxRepo:             outboxRepo,
		storage:                storage,
		cache:                  cache,
		publisher:              publisher,
		stuckUploadTimeout:     stuckUploadTimeout,
		stuckProcessingTimeout: stuckProcessingTimeout,
	}
}

// Run executes the full reconciliation cycle. Called by the scheduler.
func (w *Worker) Run(ctx context.Context) {
	// Acquire a distributed lock so only one replica runs reconciliation at a time.
	token, acquired, err := w.cache.AcquireLock(ctx, "reconciliation:run", 5*time.Minute)
	if err != nil {
		slog.Error("Reconciliation: failed to acquire lock", "error", err)
		return
	}
	if !acquired {
		slog.Debug("Reconciliation: another replica is already running, skipping")
		return
	}
	defer w.cache.ReleaseLock(ctx, "reconciliation:run", token)

	slog.Info("Reconciliation: starting cycle")
	w.reconcileExpiredUploads(ctx)
	w.reconcileOutboxFailures(ctx)
	slog.Info("Reconciliation: cycle complete")
}

// reconcileExpiredUploads aborts upload sessions that have passed their TTL.
// These arise when clients crash or disconnect during upload.
func (w *Worker) reconcileExpiredUploads(ctx context.Context) {
	expiredBefore := time.Now().Add(-w.stuckUploadTimeout)
	sessions, err := w.uploadRepo.ListExpired(ctx, expiredBefore)
	if err != nil {
		slog.Error("Reconciliation: failed to list expired uploads", "error", err)
		return
	}

	for _, session := range sessions {
		slog.Info("Reconciliation: aborting expired upload",
			"uploadId", session.UploadID,
			"mediaId", session.MediaID,
			"expiresAt", session.ExpiresAt,
		)

		// Abort S3 multipart upload
		if abortErr := w.storage.AbortMultipartUpload(ctx, session.ObjectKey, session.S3UploadID); abortErr != nil {
			slog.Error("Reconciliation: failed to abort S3 upload", "objectKey", session.ObjectKey, "error", abortErr)
		}

		// Mark session as EXPIRED
		if updateErr := w.uploadRepo.UpdateStatus(ctx, session.UploadID, domain.UploadStatusExpired); updateErr != nil {
			slog.Error("Reconciliation: failed to mark session expired", "uploadId", session.UploadID, "error", updateErr)
		}

		// Mark media as ABORTED
		if updateErr := w.mediaRepo.UpdateStatus(ctx, session.MediaID, domain.MediaStatusUploading, domain.MediaStatusAborted); updateErr != nil {
			slog.Error("Reconciliation: failed to mark media aborted", "mediaId", session.MediaID, "error", updateErr)
		}
	}

	if len(sessions) > 0 {
		slog.Info("Reconciliation: expired uploads processed", "count", len(sessions))
	}
}

// reconcileOutboxFailures retries FAILED outbox events.
func (w *Worker) reconcileOutboxFailures(ctx context.Context) {
	events, err := w.outboxRepo.ListPending(ctx, 20)
	if err != nil {
		slog.Error("Reconciliation: failed to list outbox events", "error", err)
		return
	}

	for _, event := range events {
		if pubErr := w.publisher.Publish(ctx, event.EventType, event.AggregateID, event.Payload, event.TraceID); pubErr != nil {
			slog.Error("Reconciliation: retry publish failed", "eventId", event.EventID, "error", pubErr)
			continue
		}
		_ = w.outboxRepo.MarkPublished(ctx, event.EventID)
	}
}
