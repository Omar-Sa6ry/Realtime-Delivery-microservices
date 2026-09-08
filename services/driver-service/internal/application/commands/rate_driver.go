package commands

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// RateDriverCommand represents the command to rate a driver.
type RateDriverCommand struct {
	DriverID   string
	UserID     string
	DeliveryID string
	Rating     float64
	Comment    string
}

// RateDriverHandler handles the RateDriverCommand.
type RateDriverHandler struct {
	driverRepo ports.DriverRepository
	reviewRepo ports.ReviewRepository
}

// NewRateDriverHandler creates a new RateDriverHandler.
func NewRateDriverHandler(driverRepo ports.DriverRepository, reviewRepo ports.ReviewRepository) *RateDriverHandler {
	return &RateDriverHandler{
		driverRepo: driverRepo,
		reviewRepo: reviewRepo,
	}
}

// Execute handles the rate driver process.
func (h *RateDriverHandler) Execute(ctx context.Context, cmd RateDriverCommand) (*domain.Review, error) {
	// 1. Verify driver exists
	_, err := h.driverRepo.FindByID(ctx, cmd.DriverID)
	if err != nil {
		return nil, err
	}

	// 2. Check if a review already exists for this delivery to prevent duplicate reviews
	existingReview, err := h.reviewRepo.FindByDeliveryID(ctx, cmd.DeliveryID)
	if err != nil {
		return nil, err
	}
	if existingReview != nil {
		return nil, domain.ErrDuplicateReview
	}

	// 3. Create Review Entity
	review, err := domain.NewReview(cmd.DriverID, cmd.UserID, cmd.DeliveryID, cmd.Rating, cmd.Comment)
	if err != nil {
		return nil, err
	}

	// 4. Save Review
	if err := h.reviewRepo.Create(ctx, review); err != nil {
		return nil, err
	}

	// 5. Calculate new average rating
	avgRating, totalReviews, err := h.reviewRepo.CalculateAverageRating(ctx, cmd.DriverID)
	if err != nil {
		return review, nil
	}

	// 6. Update Driver with new rating
	err = h.driverRepo.UpdateRating(ctx, cmd.DriverID, avgRating, totalReviews)
	if err != nil {
		return review, nil // the review is saved, but driver cache/rating wasn't updated.
	}

	return review, nil
}
