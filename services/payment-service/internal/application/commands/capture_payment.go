package commands

import (
	"context"

	"github.com/realtime-delivery/payment-service/internal/application/services"
	"github.com/realtime-delivery/payment-service/internal/domain"
)

// CapturePaymentCommand represents the command to capture a payment.
type CapturePaymentCommand struct {
	PaymentID string
	Amount    int64
}

// Handle executes the capture payment command.
func (c *CapturePaymentCommand) Handle(
	service *services.PaymentService,
) (*domain.Payment, error) {
	return service.CapturePayment(
		context.Background(),
		c.PaymentID,
		c.Amount,
	)
}

// CapturePaymentCommandFactory creates a CapturePaymentCommand.
func CapturePaymentCommandFactory(paymentID string, amount int64) *CapturePaymentCommand {
	return &CapturePaymentCommand{
		PaymentID: paymentID,
		Amount:    amount,
	}
}