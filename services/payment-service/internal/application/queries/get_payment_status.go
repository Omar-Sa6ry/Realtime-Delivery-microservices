package queries

import (
	"context"

	"github.com/realtime-delivery/payment-service/internal/application/services"
	"github.com/realtime-delivery/payment-service/internal/domain"
)

// GetPaymentStatusQuery represents the query to retrieve payment status.
type GetPaymentStatusQuery struct {
	PaymentID string
}

// Handle executes the get payment status query.
func (q *GetPaymentStatusQuery) Handle(
	service *services.PaymentService,
) (string, error) {
	return service.GetPaymentStatus(
		context.Background(),
		q.PaymentID,
	)
}

// GetPaymentStatusQueryFactory creates a GetPaymentStatusQuery.
func GetPaymentStatusQueryFactory(paymentID string) *GetPaymentStatusQuery {
	return &GetPaymentStatusQuery{
		PaymentID: paymentID,
	}
}