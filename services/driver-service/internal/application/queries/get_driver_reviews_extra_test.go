package queries_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/queries"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

// reviewRepoWithFindError injects a failure into FindByDriverID, which the
// shared mock cannot produce (SetError only affects Create).
type reviewRepoWithFindError struct {
	*mocks.MockReviewRepository
	findErr error
}

func (r *reviewRepoWithFindError) FindByDriverID(ctx context.Context, driverID string, skip, limit int) ([]*domain.Review, int64, error) {
	if r.findErr != nil {
		return nil, 0, r.findErr
	}
	return r.MockReviewRepository.FindByDriverID(ctx, driverID, skip, limit)
}

func TestGetDriverReviewsHandler_ErrorAndPaginationDefaults(t *testing.T) {
	t.Run("driver repo error", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetError(errors.New("mongo down"))
		handler := queries.NewGetDriverReviewsHandler(mocks.NewMockReviewRepository(), driverRepo)

		result, err := handler.Execute(context.Background(), queries.GetDriverReviewsQuery{DriverID: "driver-1", Page: 1, Limit: 10})
		if err == nil || err.Error() != "mongo down" {
			t.Errorf("Execute() error = %v, want mongo down", err)
		}
		if result != nil {
			t.Errorf("Execute() result = %+v, want nil", result)
		}
	})

	t.Run("review repo error", func(t *testing.T) {
		driverRepo := mocks.NewMockDriverRepository()
		driverRepo.SetDriver(&domain.Driver{ID: "driver-1", Status: domain.DriverStatusAvailable, IsActive: true})
		reviewRepo := &reviewRepoWithFindError{MockReviewRepository: mocks.NewMockReviewRepository(), findErr: errors.New("mongo down")}
		handler := queries.NewGetDriverReviewsHandler(reviewRepo, driverRepo)

		result, err := handler.Execute(context.Background(), queries.GetDriverReviewsQuery{DriverID: "driver-1", Page: 1, Limit: 10})
		if err == nil || err.Error() != "mongo down" {
			t.Errorf("Execute() error = %v, want mongo down", err)
		}
		if result != nil {
			t.Errorf("Execute() result = %+v, want nil", result)
		}
	})

	t.Run("page and limit defaults and clamping", func(t *testing.T) {
		tests := []struct {
			name        string
			page        int
			limit       int
			wantPage    int
			wantLimit   int
		}{
			{name: "page < 1 becomes 1", page: 0, limit: 10, wantPage: 1, wantLimit: 10},
			{name: "negative limit becomes 10", page: 1, limit: -3, wantPage: 1, wantLimit: 10},
			{name: "limit > 50 clamped to 50", page: 2, limit: 100, wantPage: 2, wantLimit: 50},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				driverRepo := mocks.NewMockDriverRepository()
				driverRepo.SetDriver(&domain.Driver{ID: "driver-1", Status: domain.DriverStatusAvailable, IsActive: true})
				handler := queries.NewGetDriverReviewsHandler(mocks.NewMockReviewRepository(), driverRepo)

				result, err := handler.Execute(context.Background(), queries.GetDriverReviewsQuery{
					DriverID: "driver-1",
					Page:     tt.page,
					Limit:    tt.limit,
				})
				if err != nil {
					t.Fatalf("Execute() unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("Execute() returned nil result")
				}
				if result.CurrentPage != tt.wantPage {
					t.Errorf("CurrentPage = %d, want %d", result.CurrentPage, tt.wantPage)
				}
				if result.Limit != tt.wantLimit {
					t.Errorf("Limit = %d, want %d", result.Limit, tt.wantLimit)
				}
			})
		}
	})
}
