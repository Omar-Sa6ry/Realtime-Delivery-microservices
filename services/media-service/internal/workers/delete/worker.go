package delete

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	kafkago "github.com/segmentio/kafka-go"
	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// deletePayload is the expected payload structure from the delete event.
type deletePayload struct {
	MediaID   string `json:"mediaId"`
	UserID    string `json:"userId"`
	ObjectKey string `json:"objectKey"`
}

// Worker processes delete events from Kafka.
// It deletes all S3 objects (original + all versions) and updates DynamoDB.
type Worker struct {
	mediaRepo   ports.MediaRepository
	versionRepo ports.VersionRepository
	quotaRepo   ports.QuotaRepository
	storage     ports.ObjectStorage
	publisher   ports.EventPublisher
}

// NewWorker creates a new delete worker.
func NewWorker(
	mediaRepo ports.MediaRepository,
	versionRepo ports.VersionRepository,
	quotaRepo ports.QuotaRepository,
	storage ports.ObjectStorage,
	publisher ports.EventPublisher,
) *Worker {
	return &Worker{
		mediaRepo:   mediaRepo,
		versionRepo: versionRepo,
		quotaRepo:   quotaRepo,
		storage:     storage,
		publisher:   publisher,
	}
}

// Handle processes a single Kafka message for media deletion.
func (w *Worker) Handle(ctx context.Context, msg kafkago.Message) error {
	var env kafkaadapter.EventEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		return fmt.Errorf("%w: unmarshal delete event: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	var payload deletePayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return fmt.Errorf("%w: unmarshal delete payload: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	return w.processDelete(ctx, payload, env.TraceID)
}

// processDelete performs the actual deletion:
// 1. List all S3 objects under the media prefix
// 2. Batch delete from S3
// 3. Update DynamoDB status to DELETED
// 4. Release quota
// 5. Publish media.deleted event
func (w *Worker) processDelete(ctx context.Context, payload deletePayload, traceID string) error {
	slog.Info("Delete worker: processing", "mediaId", payload.MediaID, "userId", payload.UserID)

	// Fetch media to get size before deletion (needed for quota)
	media, err := w.mediaRepo.GetByID(ctx, payload.MediaID)
	if err != nil {
		return fmt.Errorf("fetch media for deletion: %w", err)
	}

	// Build list of all object keys to delete (original + all versions)
	prefix := buildPrefix(payload.UserID, media.MediaID)
	objectKeys, err := w.storage.ListObjectsWithPrefix(ctx, prefix)
	if err != nil {
		return fmt.Errorf("list objects for deletion prefix %q: %w", prefix, err)
	}

	if len(objectKeys) > 0 {
		if err := w.storage.DeleteObjects(ctx, objectKeys); err != nil {
			return fmt.Errorf("S3 batch delete for media %q: %w", payload.MediaID, err)
		}
		slog.Info("Delete worker: S3 objects deleted", "mediaId", payload.MediaID, "count", len(objectKeys))
	}

	// Mark media as DELETED in DynamoDB
	if err := w.mediaRepo.UpdateStatus(ctx, payload.MediaID, domain.MediaStatusDeleting, domain.MediaStatusDeleted); err != nil {
		slog.Error("Delete worker: failed to mark media DELETED", "mediaId", payload.MediaID, "error", err)
	}

	// Release storage quota
	if releaseErr := w.quotaRepo.SubtractUsedBytes(ctx, payload.UserID, media.Size); releaseErr != nil {
		slog.Error("Delete worker: failed to release quota", "userId", payload.UserID, "error", releaseErr)
	}

	// Publish media.deleted event
	eventBytes, _ := kafkaadapter.MarshalEnvelope("", kafkaadapter.TopicMediaDeleted, traceID,
		map[string]interface{}{"mediaId": payload.MediaID, "userId": payload.UserID})
	if pubErr := w.publisher.Publish(ctx, kafkaadapter.TopicMediaDeleted, payload.MediaID, eventBytes, traceID); pubErr != nil {
		slog.Error("Delete worker: failed to publish media.deleted event", "mediaId", payload.MediaID, "error", pubErr)
	}

	slog.Info("Delete worker: media deletion complete", "mediaId", payload.MediaID)
	return nil
}

func buildPrefix(userID, mediaID string) string {
	return fmt.Sprintf("users/%s/media/%s/", userID, mediaID)
}
