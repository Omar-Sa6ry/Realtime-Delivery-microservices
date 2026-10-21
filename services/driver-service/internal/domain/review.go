package domain

import (
	"time"

	"github.com/google/uuid"
)

type Review struct {
	ID         string
	DriverID   string
	UserID     string
	DeliveryID string
	Rating     float64
	Comment    string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewReview(driverID, userID, deliveryID string, rating float64, comment string) (*Review, error) {
	if driverID == "" || userID == "" || deliveryID == "" {
		return nil, ErrInvalidArgument
	}
	if rating < 1.0 || rating > 5.0 {
		return nil, ErrInvalidRating
	}

	return &Review{
		ID:         uuid.New().String(),
		DriverID:   driverID,
		UserID:     userID,
		DeliveryID: deliveryID,
		Rating:     rating,
		Comment:    comment,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}
