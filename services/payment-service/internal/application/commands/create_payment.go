package commands

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/application/services"
)

type CreatePaymentCommand struct {
	DeliveryID     string
	UserID         string
	AmountMinor    int64
	Currency       string
	IdempotencyKey string
	CorrelationID  string
	CausationID    string
}

func (c *CreatePaymentCommand) Handle(
	ctx context.Context,
	service *services.PaymentService,
) (*services.CreatePaymentResult, error) {
	return service.CreatePayment(
		ctx,
		services.CreatePaymentInput{
			DeliveryID:     c.DeliveryID,
			UserID:         c.UserID,
			AmountMinor:    c.AmountMinor,
			Currency:       c.Currency,
			IdempotencyKey: c.IdempotencyKey,
			CorrelationID:  c.CorrelationID,
			CausationID:    c.CausationID,
		},
	)
}

func CreatePaymentCommandFactory(deliveryID, userID string, amountMinor int64, currency, idempotencyKey, correlationID, causationID string) *CreatePaymentCommand {
	return &CreatePaymentCommand{
		DeliveryID:     deliveryID,
		UserID:         userID,
		AmountMinor:    amountMinor,
		Currency:       currency,
		IdempotencyKey: idempotencyKey,
		CorrelationID:  correlationID,
		CausationID:    causationID,
	}
}
