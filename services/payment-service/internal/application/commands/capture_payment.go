package commands

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

type CapturePaymentCommand struct {
	PaymentID      string
	Amount         int64
	IdempotencyKey string
	CorrelationID  string
}

func (c *CapturePaymentCommand) Handle(
	service *services.PaymentService,
) (*domain.Payment, error) {
	return service.CapturePayment(
		context.Background(),
		services.CapturePaymentInput{
			PaymentID:      c.PaymentID,
			AmountMinor:    c.Amount,
			IdempotencyKey: c.IdempotencyKey,
			CorrelationID:  c.CorrelationID,
		},
	)
}

func CapturePaymentCommandFactory(paymentID string, amount int64, idempotencyKey, correlationID string) *CapturePaymentCommand {
	return &CapturePaymentCommand{
		PaymentID:      paymentID,
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
		CorrelationID:  correlationID,
	}
}
