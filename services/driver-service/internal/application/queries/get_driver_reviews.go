package queries

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// GetDriverReviewsQuery retrieves driver reviews with pagination and stats.
type GetDriverReviewsQuery struct {
	DriverID string
	Page     int
	Limit    int
}

// DriverReviewsResult encapsulates the reviews and pagination stats.
type DriverReviewsResult struct {
	Reviews       []*domain.Review
	TotalItems    int64
	AverageRating float64
	TotalReviews  int64
	CurrentPage   int
	Limit         int
}

// GetDriverReviewsHandler handles the GetDriverReviewsQuery.
type GetDriverReviewsHandler struct {
	reviewRepo ports.ReviewRepository
	driverRepo ports.DriverRepository
}

// NewGetDriverReviewsHandler creates a new handler.
func NewGetDriverReviewsHandler(reviewRepo ports.ReviewRepository, driverRepo ports.DriverRepository) *GetDriverReviewsHandler {
	return &GetDriverReviewsHandler{
		reviewRepo: reviewRepo,
		driverRepo: driverRepo,
	}
}

// Execute returns the reviews for a driver.
func (h *GetDriverReviewsHandler) Execute(ctx context.Context, query GetDriverReviewsQuery) (*DriverReviewsResult, error) {
	// Verify driver exists
	driver, err := h.driverRepo.FindByID(ctx, query.DriverID)
	if err != nil {
		return nil, err
	}
	if driver == nil {
		return nil, domain.ErrDriverNotFound
	}

	page := query.Page
	if page < 1 {
		page = 1
	}
	limit := query.Limit
	if limit < 1 {
		limit = 10
	} else if limit > 50 {
		limit = 50
	}

	skip := (page - 1) * limit

	reviews, totalItems, err := h.reviewRepo.FindByDriverID(ctx, query.DriverID, skip, limit)
	if err != nil {
		return nil, err
	}

	return &DriverReviewsResult{
		Reviews:       reviews,
		TotalItems:    totalItems,
		AverageRating: driver.Rating,
		TotalReviews:  driver.RatingCount,
		CurrentPage:   page,
		Limit:         limit,
	}, nil
}
