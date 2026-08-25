package grpc

import (
	"context"

	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/download"
	appMedia "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/media"
	pb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/grpc/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	metadataRequesterID = "x-requester-id"
	metadataTraceID     = "x-correlation-id"
)

type Server struct {
	pb.UnimplementedMediaServiceServer
	getDownloadURL *download.GetDownloadUrlUseCase
	getMedia       *appMedia.GetMediaUseCase
	errorMapper    *errorMapperRegistry
}

func NewServer(
	getDownloadURL *download.GetDownloadUrlUseCase,
	getMedia *appMedia.GetMediaUseCase,
) *Server {
	return &Server{
		getDownloadURL: getDownloadURL,
		getMedia:       getMedia,
		errorMapper:    BuildErrorMapperRegistry(),
	}
}

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
		return nil, toGRPCStatus(err, s.errorMapper)
	}

	return &pb.ResolveMediaUrlResponse{
		Url:              out.URL,
		ExpiresAtSeconds: out.ExpiresAt.Unix(),
		ContentType:      out.ContentType,
		MediaId:          in.GetMediaId(),
		Status:           "", // set below by caller if needed (use GetMedia for status)
	}, nil
}

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
		return nil, toGRPCStatus(err, s.errorMapper)
	}

	m := out.Media
	return &pb.GetMediaResponse{
		MediaId:           m.MediaID,
		OwnerId:           m.OwnerID,
		FileName:          m.FileName,
		ContentType:       m.ContentType,
		MediaType:         string(m.MediaType),
		Size:              m.Size,
		Status:            string(m.Status),
		ObjectKey:         m.ObjectKey,
		CreatedAtSeconds:  m.CreatedAt.Unix(),
		UpdatedAtSeconds:  m.UpdatedAt.Unix(),
	}, nil
}

func requesterFromCtx(ctx context.Context) string {
	return sharedlogging.GetUserID(ctx)
}