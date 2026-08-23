package graphql

import (
	"context"
	"errors"
	"fmt"

	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/download"
	appMedia "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/media"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/upload"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/graphql/dlq"
	gql "github.com/graphql-go/graphql"
)

type Handler struct {
	createSession   *upload.CreateSessionUseCase
	completeUpload  *upload.CompleteUploadUseCase
	abortUpload     *upload.AbortUploadUseCase
	getUploadStatus *upload.GetUploadStatusUseCase
	renewPresigned  *upload.RenewPresignedUseCase
	getMedia        *appMedia.GetMediaUseCase
	listMedia       *appMedia.ListMediaUseCase
	deleteMedia     *appMedia.DeleteMediaUseCase
	getDownloadURL  *download.GetDownloadUrlUseCase
	quotaRepo       ports.QuotaRepository
	dlqManager      *dlq.DLQManager
	kafkaBrokers    []string
}

func NewHandler(
	createSession *upload.CreateSessionUseCase,
	completeUpload *upload.CompleteUploadUseCase,
	abortUpload *upload.AbortUploadUseCase,
	getUploadStatus *upload.GetUploadStatusUseCase,
	renewPresigned *upload.RenewPresignedUseCase,
	getMedia *appMedia.GetMediaUseCase,
	listMedia *appMedia.ListMediaUseCase,
	deleteMedia *appMedia.DeleteMediaUseCase,
	getDownloadURL *download.GetDownloadUrlUseCase,
	quotaRepo ports.QuotaRepository,
	dlqManager *dlq.DLQManager,
	kafkaBrokers []string,
) *Handler {
	return &Handler{
		createSession:   createSession,
		completeUpload:  completeUpload,
		abortUpload:     abortUpload,
		getUploadStatus: getUploadStatus,
		renewPresigned:  renewPresigned,
		getMedia:        getMedia,
		listMedia:       listMedia,
		deleteMedia:     deleteMedia,
		getDownloadURL:  getDownloadURL,
		quotaRepo:       quotaRepo,
		dlqManager:      dlqManager,
		kafkaBrokers:    kafkaBrokers,
	}
}

func (h *Handler) requireUser(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("authentication required: missing x-user-id header")
	}
	userID := sharedlogging.GetUserID(ctx)
	if userID == "" {
		return "", errors.New("authentication required: missing x-user-id header")
	}
	return userID, nil
}

func (h *Handler) resolveMedia(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	out, err := h.getMedia.Execute(ctx, userID, argString(p.Args, "mediaId"))
	if err != nil {
		return nil, err
	}
	return mapMedia(out.Media, out.Versions), nil
}

func (h *Handler) resolveListMedia(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	items, nextCursor, err := h.listMedia.Execute(ctx, appMedia.ListMediaInput{
		OwnerID:      userID,
		Limit:        argInt(p.Args, "limit"),
		Cursor:       argString(p.Args, "cursor"),
		StatusFilter: argString(p.Args, "statusFilter"),
	})
	if err != nil {
		return nil, err
	}

	graphItems := make([]interface{}, 0, len(items))
	for _, m := range items {
		graphItems = append(graphItems, mapMedia(m, nil))
	}
	return map[string]interface{}{
		"items":      graphItems,
		"nextCursor": nextCursor,
	}, nil
}

func (h *Handler) resolveUploadStatus(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	out, err := h.getUploadStatus.Execute(ctx, userID, argString(p.Args, "uploadId"))
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"uploadId":       out.UploadID,
		"status":         string(out.Status),
		"totalParts":     out.TotalParts,
		"completedParts": out.CompletedParts,
		"missingParts":   intSlice(out.MissingParts),
		"expiresAt":      float64(out.ExpiresAt.Unix()),
	}, nil
}

func (h *Handler) resolveDownloadUrl(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	out, err := h.getDownloadURL.Execute(ctx, download.GetDownloadUrlInput{
		UserID:        userID,
		MediaID:       argString(p.Args, "mediaId"),
		VersionType:   argString(p.Args, "versionType"),
		ExpirySeconds: argInt(p.Args, "expirySeconds"),
	})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"url":         out.URL,
		"expiresAt":   float64(out.ExpiresAt.Unix()),
		"contentType": out.ContentType,
	}, nil
}

func (h *Handler) resolveQuota(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	usage, err := h.quotaRepo.GetUsage(ctx, userID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"usedBytes":            float64(usage.UsedBytes),
		"quotaBytes":           float64(usage.QuotaBytes),
		"remainingBytes":       float64(usage.RemainingBytes()),
		"activeUploads":        usage.ActiveUploads,
		"maxConcurrentUploads": usage.MaxConcurrent,
	}, nil
}

func (h *Handler) resolveCreateUploadSession(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	input, _ := p.Args["input"].(map[string]interface{})
	out, err := h.createSession.Execute(ctx, upload.CreateSessionInput{
		UserID:         userID,
		FileName:       argString(input, "fileName"),
		ContentType:    argString(input, "contentType"),
		Size:           int64(argFloat(input, "size")),
		Checksum:       argString(input, "checksum"),
		DeliveryID:     argString(input, "deliveryId"),
		IdempotencyKey: argString(input, "idempotencyKey"),
	})
	if err != nil {
		return nil, err
	}

	parts := make([]interface{}, 0, len(out.PresignedParts))
	for _, part := range out.PresignedParts {
		parts = append(parts, map[string]interface{}{
			"partNumber":   part.PartNumber,
			"presignedUrl": part.PresignedURL,
		})
	}
	return map[string]interface{}{
		"mediaId":        out.MediaID,
		"uploadId":       out.UploadID,
		"s3UploadId":     out.S3UploadID,
		"presignedParts": parts,
		"partSize":       float64(out.PartSize),
		"totalParts":     out.TotalParts,
		"expiresAt":      float64(out.ExpiresAt.Unix()),
	}, nil
}

func (h *Handler) resolveCompleteUpload(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	input, _ := p.Args["input"].(map[string]interface{})
	partsRaw, _ := input["parts"].([]interface{})
	parts := make([]domain.UploadPart, 0, len(partsRaw))
	for _, pr := range partsRaw {
		pm, _ := pr.(map[string]interface{})
		parts = append(parts, domain.UploadPart{
			PartNumber: argInt(pm, "partNumber"),
			ETag:       argString(pm, "eTag"),
		})
	}

	out, err := h.completeUpload.Execute(ctx, upload.CompleteUploadInput{
		UserID:         userID,
		UploadID:       argString(input, "uploadId"),
		Parts:          parts,
		IdempotencyKey: argString(input, "idempotencyKey"),
	})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"mediaId": out.MediaID,
		"status":  string(out.Status),
	}, nil
}

func (h *Handler) resolveAbortUpload(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.abortUpload.Execute(ctx, userID, argString(p.Args, "uploadId")); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true}, nil
}

func (h *Handler) resolveDeleteMedia(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.deleteMedia.Execute(
		ctx, userID, argString(p.Args, "mediaId"), argString(p.Args, "idempotencyKey"),
	); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true}, nil
}

func mapMedia(m *domain.Media, versions []*domain.MediaVersion) map[string]interface{} {
	if m == nil {
		return nil
	}

	versionMaps := make([]interface{}, 0, len(versions))
	for _, v := range versions {
		if v == nil {
			continue
		}
		versionMaps = append(versionMaps, map[string]interface{}{
			"versionType": string(v.VersionType),
			"objectKey":   v.ObjectKey,
			"contentType": v.ContentType,
			"size":        float64(v.Size),
			"width":       int(v.Width),
			"height":      int(v.Height),
			"durationMs":  float64(v.DurationMS),
		})
	}

	return map[string]interface{}{
		"mediaId":     m.MediaID,
		"ownerId":     m.OwnerID,
		"fileName":    m.FileName,
		"contentType": m.ContentType,
		"mediaType":   string(m.MediaType),
		"size":        float64(m.Size),
		"status":      string(m.Status),
		"objectKey":   m.ObjectKey,
		"createdAt":   float64(m.CreatedAt.Unix()),
		"updatedAt":   float64(m.UpdatedAt.Unix()),
		"versions":    versionMaps,
	}
}

func argString(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func argInt(args map[string]interface{}, key string) int {
	switch v := args[key].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func argFloat(args map[string]interface{}, key string) float64 {
	switch v := args[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func intSlice(in []int) []interface{} {
	out := make([]interface{}, 0, len(in))
	for _, v := range in {
		out = append(out, v)
	}
	return out
}

// resolveDLQTopics returns all DLQ topics
func (h *Handler) resolveDLQTopics(p gql.ResolveParams) (interface{}, error) {
	return h.dlqManager.ListDLQTopics(), nil
}

// resolveDLQStats returns statistics for DLQ topics
func (h *Handler) resolveDLQStats(p gql.ResolveParams) (interface{}, error) {
	topicsArg, ok := p.Args["topics"].([]interface{})
	var topics []string
	if ok {
		for _, t := range topicsArg {
			if s, ok := t.(string); ok {
				topics = append(topics, s)
			}
		}
	}
	if len(topics) == 0 {
		topics = h.dlqManager.ListDLQTopics()
	}

	stats, err := h.dlqManager.GetDLQStats(context.Background(), h.kafkaBrokers, topics)
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, 0, len(stats))
	for topic, count := range stats {
		result = append(result, map[string]interface{}{
			"topic":        topic,
			"messageCount": count,
		})
	}
	return result, nil
}

// resolveDLQReplay replays messages from a DLQ topic.
func (h *Handler) resolveDLQReplay(p gql.ResolveParams) (interface{}, error) {
	topic := argString(p.Args, "topic")
	if topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	maxMessages := argInt(p.Args, "maxMessages")
	if maxMessages <= 0 {
		maxMessages = 100
	}
	replayed, err := h.dlqManager.ReplayAllDLQMessages(p.Context, h.kafkaBrokers, topic, maxMessages)
	if err != nil {
		return map[string]interface{}{"success": false, "replayedCount": 0, "errors": []string{err.Error()}}, nil
	}
	return map[string]interface{}{"success": true, "replayedCount": replayed, "errors": []string{}}, nil
}

// resolveRenewPresigned renews expired multipart URLs for the authenticated owner.
func (h *Handler) resolveRenewPresigned(p gql.ResolveParams) (interface{}, error) {
	inputArg, ok := p.Args["input"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid input")
	}
	userID, err := h.requireUser(p.Context)
	if err != nil {
		return nil, err
	}
	uploadID := argString(inputArg, "uploadId")
	if uploadID == "" {
		return nil, fmt.Errorf("uploadId is required")
	}
	result, err := h.renewPresigned.Execute(p.Context, upload.RenewPresignedInput{
		UserID: userID, UploadID: uploadID, ExpirySeconds: argInt(inputArg, "expirySeconds"),
	})
	if err != nil {
		return nil, err
	}
	parts := make([]interface{}, len(result.PresignedParts))
	for i, part := range result.PresignedParts {
		parts[i] = map[string]interface{}{"partNumber": part.PartNumber, "presignedUrl": part.PresignedURL}
	}
	return map[string]interface{}{
		"uploadId": result.UploadID, "s3UploadId": result.S3UploadID,
		"presignedParts": parts, "partSize": float64(result.PartSize),
		"totalParts": result.TotalParts, "expiresAt": float64(result.ExpiresAt.Unix()),
	}, nil
}
