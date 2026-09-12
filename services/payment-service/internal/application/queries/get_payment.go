package queries

import (
	"context"

	"github.com/realtime-delivery/payment-service/internal/application/services"
	"github.com/realtime-delivery/payment-service/internal/domain"
)

// GetPaymentQuery represents the query to retrieve a payment.
type GetPaymentQuery struct {
	PaymentID string
}

// Handle executes the get payment query.
func (q *GetPaymentQuery) Handle(
	service *services.PaymentService,
) (*domain.Payment, error) {
	return service.GetPayment(
		context.Background(),
		q.PaymentID,
	)
}

// GetPaymentQueryFactory creates a GetPaymentQuery.
func GetPaymentQueryFactory(paymentID string) *GetPaymentQuery {
	return &GetPaymentQuery{
		PaymentID: paymentID,
	}
}