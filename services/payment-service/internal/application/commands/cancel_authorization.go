package commands

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

type CancelAuthorizationCommand struct {
	PaymentID      string
	IdempotencyKey string
}

func (c *CancelAuthorizationCommand) Handle(
	service *services.PaymentService,
) (*domain.Payment, error) {
	return service.CancelAuthorization(
		context.Background(),
		c.PaymentID,
		c.IdempotencyKey,
	)
}

func CancelAuthorizationCommandFactory(paymentID, idempotencyKey string) *CancelAuthorizationCommand {
	return &CancelAuthorizationCommand{
		PaymentID:      paymentID,
		IdempotencyKey: idempotencyKey,
	}
}
