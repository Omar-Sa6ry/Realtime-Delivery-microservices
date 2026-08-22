package indexing

import (
	"context"
	"log/slog"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
)

type Service struct {
	repo search.SearchRepository
}

func NewService(repo search.SearchRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) UpsertDelivery(ctx context.Context, doc search.DeliveryDocument) error {
	slog.Info("Indexing delivery", "id", doc.DeliveryID, "status", doc.Status, "version", doc.SourceVersion)
	return s.repo.UpsertDelivery(ctx, doc)
}

func (s *Service) DeleteDelivery(ctx context.Context, id string) error {
	slog.Info("Deleting delivery from index", "id", id)
	return s.repo.DeleteDelivery(ctx, id)
}

func (s *Service) UpsertDriver(ctx context.Context, doc search.DriverDocument) error {
	slog.Info("Indexing driver", "id", doc.DriverID, "status", doc.Status, "version", doc.SourceVersion)
	return s.repo.UpsertDriver(ctx, doc)
}

func (s *Service) DeleteDriver(ctx context.Context, id string) error {
	slog.Info("Deleting driver from index", "id", id)
	return s.repo.DeleteDriver(ctx, id)
}

func (s *Service) UpsertMedia(ctx context.Context, doc search.MediaDocument) error {
	slog.Info("Indexing media", "id", doc.MediaID, "status", doc.Status, "version", doc.SourceVersion)
	return s.repo.UpsertMedia(ctx, doc)
}

func (s *Service) DeleteMedia(ctx context.Context, id string) error {
	slog.Info("Deleting media from index", "id", id)
	return s.repo.DeleteMedia(ctx, id)
}
