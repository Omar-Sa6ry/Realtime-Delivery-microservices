package grpc

import (
	"context"
	"fmt"

	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/download"
	appMedia "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/media"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/upload"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	mediav1 "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/proto/media/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure Handler implements the generated MediaServiceServer interface.
var _ mediav1.MediaServiceServer = (*Handler)(nil)

// CreateUploadSession initiates a Direct-to-S3 multipart upload session.
func (h *Handler) CreateUploadSession(ctx context.Context, req *mediav1.CreateUploadSessionRequest) (*mediav1.CreateUploadSessionResponse, error) {
	// Extract user ID from context (injected by interceptor)
	userID := sharedlogging.GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	out, err := h.createSession.Execute(ctx, upload.CreateSessionInput{
		UserID:         userID,
		FileName:       req.FileName,
		ContentType:    req.ContentType,
		Size:           req.Size,
		Checksum:        req.Checksum,
		DeliveryID:     req.DeliveryId,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, mapErrorToGRPC(err)
	}

	parts := make([]*mediav1.PresignedPart, len(out.PresignedParts))
	for i, p := range out.PresignedParts {
		parts[i] = &mediav1.PresignedPart{
			PartNumber:   int32(p.PartNumber),
			PresignedUrl: p.PresignedURL,
		}
	}

	return &mediav1.CreateUploadSessionResponse{
		MediaId:        out.MediaID,
		UploadId:       out.UploadID,
		S3UploadId:     out.S3UploadID,
		PresignedParts: parts,
		PartSize:       out.PartSize,
		TotalParts:     int32(out.TotalParts),
		ExpiresAt:      out.ExpiresAt.Unix(),
	}, nil
}

// GetUploadStatus retrieves the part completion status of an in-progress session.
func (h *Handler) GetUploadStatus(ctx context.Context, req *mediav1.GetUploadStatusRequest) (*mediav1.GetUploadStatusResponse, error) {
	userID := sharedlogging.GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	out, err := h.getUploadStatus.Execute(ctx, userID, req.UploadId)
	if err != nil {
		return nil, mapErrorToGRPC(err)
	}

	missing := make([]int32, len(out.MissingParts))
	for i, p := range out.MissingParts {
		missing[i] = int32(p)
	}

	return &mediav1.GetUploadStatusResponse{
		UploadId:       out.UploadID,
		Status:         string(out.Status),
		TotalParts:     int32(out.TotalParts),
		CompletedParts: int32(out.CompletedParts),
		MissingParts:   missing,
		ExpiresAt:      out.ExpiresAt.Unix(),
	}, nil
}

// CompleteUpload finalizes the multipart upload in S3 and assemblies the object.
func (h *Handler) CompleteUpload(ctx context.Context, req *mediav1.CompleteUploadRequest) (*mediav1.CompleteUploadResponse, error) {
	userID := sharedlogging.GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	parts := make([]domain.UploadPart, len(req.Parts))
	for i, p := range req.Parts {
		parts[i] = domain.UploadPart{
			PartNumber: int(p.PartNumber),
			ETag:       p.ETag,
		}
	}

	out, err := h.completeUpload.Execute(ctx, upload.CompleteUploadInput{
		UserID:         userID,
		UploadID:       req.UploadId,
		Parts:          parts,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, mapErrorToGRPC(err)
	}

	return &mediav1.CompleteUploadResponse{
		MediaId: out.MediaID,
		Status:  string(out.Status),
	}, nil
}

// AbortUpload cancels the upload session and frees S3 multipart parts.
func (h *Handler) AbortUpload(ctx context.Context, req *mediav1.AbortUploadRequest) (*mediav1.AbortUploadResponse, error) {
	userID := sharedlogging.GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	err := h.abortUpload.Execute(ctx, userID, req.UploadId)
	if err != nil {
		return nil, mapErrorToGRPC(err)
	}

	return &mediav1.AbortUploadResponse{Success: true}, nil
}

// GetMedia fetches metadata for a specific media item.
func (h *Handler) GetMedia(ctx context.Context, req *mediav1.GetMediaRequest) (*mediav1.GetMediaResponse, error) {
	userID := sharedlogging.GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	out, err := h.getMedia.Execute(ctx, userID, req.MediaId)
	if err != nil {
		return nil, mapErrorToGRPC(err)
	}

	return &mediav1.GetMediaResponse{
		Media: mapToProtoMedia(out.Media, out.Versions),
	}, nil
}

// ListMedia lists paginated media items owned by the caller.
func (h *Handler) ListMedia(ctx context.Context, req *mediav1.ListMediaRequest) (*mediav1.ListMediaResponse, error) {
	// For queries, list by ownerID provided in req (or auth user)
	userID := sharedlogging.GetUserID(ctx)
	if userID == "" {
		userID = req.OwnerId
	}

	// Fetch without status filter for now
	items, nextCursor, err := h.listMedia.Execute(ctx, appMedia.ListMediaInput{
		OwnerID: userID,
		Limit:   int(req.Limit),
		Cursor:  req.Cursor,
	})
	if err != nil {
		return nil, mapErrorToGRPC(err)
	}

	protoItems := make([]*mediav1.MediaInfo, len(items))
	for i, it := range items {
		protoItems[i] = mapToProtoMedia(it, nil)
	}

	return &mediav1.ListMediaResponse{
		Items:      protoItems,
		NextCursor: nextCursor,
	}, nil
}

// GetDownloadUrl returns a time-limited download URL for a file rendition.
func (h *Handler) GetDownloadUrl(ctx context.Context, req *mediav1.GetDownloadUrlRequest) (*mediav1.GetDownloadUrlResponse, error) {
	userID := sharedlogging.GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	out, err := h.getDownloadURL.Execute(ctx, download.GetDownloadUrlInput{
		UserID:        userID,
		MediaID:       req.MediaId,
		VersionType:   req.VersionType,
		ExpirySeconds: int(req.ExpirySeconds),
	})
	if err != nil {
		return nil, mapErrorToGRPC(err)
	}

	return &mediav1.GetDownloadUrlResponse{
		Url:         out.URL,
		ExpiresAt:   out.ExpiresAt.Unix(),
		ContentType: out.ContentType,
	}, nil
}

// DeleteMedia marks a file for deletion (async worker deletes S3 objects).
func (h *Handler) DeleteMedia(ctx context.Context, req *mediav1.DeleteMediaRequest) (*mediav1.DeleteMediaResponse, error) {
	userID := sharedlogging.GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	err := h.deleteMedia.Execute(ctx, userID, req.MediaId, req.IdempotencyKey)
	if err != nil {
		return nil, mapErrorToGRPC(err)
	}

	return &mediav1.DeleteMediaResponse{Accepted: true}, nil
}

// GetQuota returns the user's storage limits and current consumption.
func (h *Handler) GetQuota(ctx context.Context, req *mediav1.GetQuotaRequest) (*mediav1.GetQuotaResponse, error) {
	userID := sharedlogging.GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	usage, err := h.quotaRepo.GetUsage(ctx, userID)
	if err != nil {
		return nil, mapErrorToGRPC(err)
	}

	return &mediav1.GetQuotaResponse{
		UsedBytes:            usage.UsedBytes,
		QuotaBytes:           usage.QuotaBytes,
		RemainingBytes:       usage.RemainingBytes(),
		ActiveUploads:        int32(usage.ActiveUploads),
		MaxConcurrentUploads: int32(usage.MaxConcurrent),
	}, nil
}

// Helper converters

func mapToProtoMedia(m *domain.Media, versions []*domain.MediaVersion) *mediav1.MediaInfo {
	if m == nil {
		return nil
	}

	protoVers := make([]*mediav1.MediaVersionInfo, len(versions))
	for i, v := range versions {
		protoVers[i] = &mediav1.MediaVersionInfo{
			VersionType: string(v.VersionType),
			ObjectKey:   v.ObjectKey,
			ContentType: v.ContentType,
			Size:        v.Size,
			Width:       v.Width,
			Height:      v.Height,
			DurationMs:  v.DurationMS,
		}
	}

	return &mediav1.MediaInfo{
		MediaId:     m.MediaID,
		OwnerId:     m.OwnerID,
		FileName:    m.FileName,
		ContentType: m.ContentType,
		MediaType:   string(m.MediaType),
		Size:        m.Size,
		Status:      string(m.Status),
		ObjectKey:   m.ObjectKey,
		CreatedAt:   m.CreatedAt.Unix(),
		UpdatedAt:   m.UpdatedAt.Unix(),
		Versions:    protoVers,
	}
}

func mapErrorToGRPC(err error) error {
	if err == nil {
		return nil
	}

	switch err {
	case domain.ErrMediaNotFound, domain.ErrUploadSessionNotFound, domain.ErrJobNotFound:
		return status.Error(codes.NotFound, err.Error())
	case domain.ErrUnauthorized:
		return status.Error(codes.PermissionDenied, err.Error())
	case domain.ErrInvalidFileType, domain.ErrFileTooLarge, domain.ErrFileTooSmall, domain.ErrInvalidStateTransition, domain.ErrInvalidMagicBytes, domain.ErrUploadPartsMissing:
		return status.Error(codes.InvalidArgument, err.Error())
	case domain.ErrQuotaExceeded, domain.ErrConcurrentUploadsExceeded:
		return status.Error(codes.ResourceExhausted, err.Error())
	case domain.ErrRateLimitExceeded:
		return status.Error(codes.Unavailable, err.Error()) // retryable
	case domain.ErrIdempotencyConflict:
		return status.Error(codes.AlreadyExists, err.Error())
	case domain.ErrChecksumMismatch:
		return status.Error(codes.DataLoss, err.Error())
	default:
		return status.Error(codes.Internal, fmt.Sprintf("internal service error: %v", err))
	}
}
