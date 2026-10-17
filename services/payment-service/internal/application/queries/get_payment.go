package queries

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

type GetPaymentQuery struct {
	PaymentID string
}

func (q *GetPaymentQuery) Handle(
	service *services.PaymentService,
) (*domain.Payment, error) {
	return service.GetPayment(
		context.Background(),
		q.PaymentID,
	)
}

func GetPaymentQueryFactory(paymentID string) *GetPaymentQuery {
	return &GetPaymentQuery{
		PaymentID: paymentID,
	}
}
