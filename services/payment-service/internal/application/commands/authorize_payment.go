package commands

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

type AuthorizePaymentCommand struct {
	PaymentID      string
	IdempotencyKey string
	CorrelationID  string
}

func (c *AuthorizePaymentCommand) Handle(
	service *services.PaymentService,
) (*domain.Payment, error) {
	return service.AuthorizePayment(
		context.Background(),
		services.AuthorizePaymentInput{
			PaymentID:      c.PaymentID,
			IdempotencyKey: c.IdempotencyKey,
			CorrelationID:  c.CorrelationID,
		},
	)
}

func AuthorizePaymentCommandFactory(paymentID, idempotencyKey, correlationID string) *AuthorizePaymentCommand {
	return &AuthorizePaymentCommand{
		PaymentID:      paymentID,
		IdempotencyKey: idempotencyKey,
		CorrelationID:  correlationID,
	}
}
