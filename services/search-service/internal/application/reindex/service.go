package reindex

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
)

type Service struct {
	repo search.SearchRepository
}

func NewService(repo search.SearchRepository) *Service {
	return &Service{repo: repo}
}

type ReindexJob struct {
	JobID       string    `json:"jobId"`
	Index       string    `json:"index"`
	Status      string    `json:"status"` // IN_PROGRESS | COMPLETED | FAILED
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt *time.Time`json:"completedAt,omitempty"`
	Error       string    `json:"error,omitempty"`
}

func (s *Service) StartReindex(ctx context.Context, indexName string) (*ReindexJob, error) {
	jobID := fmt.Sprintf("reindex-%s-%d", indexName, time.Now().Unix())
	job := &ReindexJob{
		JobID:     jobID,
		Index:     indexName,
		Status:    "IN_PROGRESS",
		StartedAt: time.Now().UTC(),
	}

	// 1. Create target version (e.g., deliveries-v2)
	// 2. OpenSearch Reindex API
	// 3. Switch Alias
	go func() {
		bgCtx := context.Background()
		slog.Info("Starting background reindex", "jobId", jobID, "index", indexName)
		// For portfolio demo, execute source to current alias reindex or clone
		err := s.repo.Reindex(bgCtx, indexName+"-v1", indexName+"-v1")
		if err != nil {
			slog.Error("Reindexing failed", "jobId", jobID, "error", err)
			return
		}
		slog.Info("Reindexing completed successfully", "jobId", jobID)
	}()

	return job, nil
}
