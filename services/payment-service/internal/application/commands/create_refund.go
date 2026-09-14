package commands

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

type CreateRefundCommand struct {
	PaymentID      string
	Reason         string
	Amount         int64
	IdempotencyKey string
	CorrelationID  string
}

func (c *CreateRefundCommand) Handle(
	service *services.PaymentService,
) (*domain.Refund, error) {
	return service.CreateRefund(
		context.Background(),
		services.CreateRefundInput{
			PaymentID:      c.PaymentID,
			AmountMinor:    c.Amount,
			Reason:         c.Reason,
			IdempotencyKey: c.IdempotencyKey,
			CorrelationID:  c.CorrelationID,
		},
	)
}

func CreateRefundCommandFactory(paymentID, reason string, amount int64, idempotencyKey, correlationID string) *CreateRefundCommand {
	return &CreateRefundCommand{
		PaymentID:      paymentID,
		Reason:         reason,
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
		CorrelationID:  correlationID,
	}
}
