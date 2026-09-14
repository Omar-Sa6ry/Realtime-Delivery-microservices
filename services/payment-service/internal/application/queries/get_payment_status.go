package queries

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

type GetPaymentStatusQuery struct {
	PaymentID string
}

func (q *GetPaymentStatusQuery) Handle(
	service *services.PaymentService,
) (domain.PaymentStatus, error) {
	return service.GetPaymentStatus(
		context.Background(),
		q.PaymentID,
	)
}

func GetPaymentStatusQueryFactory(paymentID string) *GetPaymentStatusQuery {
	return &GetPaymentStatusQuery{
		PaymentID: paymentID,
	}
}
