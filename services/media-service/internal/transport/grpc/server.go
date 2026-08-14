package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/download"
	appMedia "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/media"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	pb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/grpc/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mediaGrpcServiceKey is the gRPC metadata key carrying the authenticated requester id.
// The api-gateway sets x-user-id as an HTTP header; internal services forward that id
// into gRPC metadata so the media-service can enforce ownership.
const (
	metadataRequesterID = "x-requester-id"
	metadataTraceID     = "x-correlation-id"
)

// Server implements pb.MediaServiceServer.
// It reuses the existing application use-cases so authorization, rate limiting and
// presigned URL generation stay identical to the GraphQL path.
type Server struct {
	pb.UnimplementedMediaServiceServer
	getDownloadURL *download.GetDownloadUrlUseCase
	getMedia       *appMedia.GetMediaUseCase
}

// NewServer constructs the gRPC service binding to the shared use-cases.
func NewServer(
	getDownloadURL *download.GetDownloadUrlUseCase,
	getMedia *appMedia.GetMediaUseCase,
) *Server {
	return &Server{
		getDownloadURL: getDownloadURL,
		getMedia:       getMedia,
	}
}

// ResolveMediaUrl returns a short-lived presigned GET URL for a media item.
// Authorization: requesterID must equal the media's OwnerID.
func (s *Server) ResolveMediaUrl(ctx context.Context, in *pb.ResolveMediaUrlRequest) (*pb.ResolveMediaUrlResponse, error) {
	requesterID := requesterFromCtx(ctx)
	if requesterID == "" {
		requesterID = in.GetRequesterId()
	}
	if requesterID == "" {
		return nil, status.Error(codes.Unauthenticated, "missing requester id")
	}

	out, err := s.getDownloadURL.Execute(ctx, download.GetDownloadUrlInput{
		UserID:        requesterID,
		MediaID:       in.GetMediaId(),
		VersionType:   in.GetVersionType(),
		ExpirySeconds: int(in.GetExpirySeconds()),
	})
	if err != nil {
		return nil, toGRPCStatus(err)
	}

	return &pb.ResolveMediaUrlResponse{
		Url:              out.URL,
		ExpiresAtSeconds: out.ExpiresAt.Unix(),
		ContentType:      out.ContentType,
		MediaId:          in.GetMediaId(),
		Status:           "", // set below by caller if needed (use GetMedia for status)
	}, nil
}

// GetMedia returns media metadata + status, enforcing ownership.
func (s *Server) GetMedia(ctx context.Context, in *pb.GetMediaRequest) (*pb.GetMediaResponse, error) {
	requesterID := requesterFromCtx(ctx)
	if requesterID == "" {
		requesterID = in.GetRequesterId()
	}
	if requesterID == "" {
		return nil, status.Error(codes.Unauthenticated, "missing requester id")
	}

	out, err := s.getMedia.Execute(ctx, requesterID, in.GetMediaId())
	if err != nil {
		return nil, toGRPCStatus(err)
	}

	m := out.Media
	return &pb.GetMediaResponse{
		MediaId:        m.MediaID,
		OwnerId:        m.OwnerID,
		FileName:       m.FileName,
		ContentType:    m.ContentType,
		MediaType:      string(m.MediaType),
		Size:           m.Size,
		Status:         string(m.Status),
		ObjectKey:      m.ObjectKey,
		CreatedAtSeconds: m.CreatedAt.Unix(),
		UpdatedAtSeconds: m.UpdatedAt.Unix(),
	}, nil
}

// requesterFromCtx extracts the requester id from gRPC metadata into the context
// set by the unary interceptor (ServerOption).
func requesterFromCtx(ctx context.Context) string {
	return sharedlogging.GetUserID(ctx)
}

// toGRPCStatus maps domain errors to gRPC status codes without leaking internals.
func toGRPCStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		return status.Error(codes.PermissionDenied, "access denied: resource ownership mismatch")
	case errors.Is(err, domain.ErrMediaNotFound):
		return status.Error(codes.NotFound, "media not found")
	case errors.Is(err, domain.ErrMediaQuarantined):
		return status.Error(codes.FailedPrecondition, "media is quarantined")
	case errors.Is(err, domain.ErrRateLimitExceeded):
		return status.Error(codes.ResourceExhausted, "rate limit exceeded")
	}
	slog.Error("grpc media use-case error", "error", err)
	return status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
}