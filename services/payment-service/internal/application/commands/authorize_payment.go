package commands

import (
	"context"

	"github.com/realtime-delivery/payment-service/internal/application/services"
	"github.com/realtime-delivery/payment-service/internal/domain"
)

// AuthorizePaymentCommand represents the command to authorize a payment.
type AuthorizePaymentCommand struct {
	PaymentID string
}

// Handle executes the authorize payment command.
func (c *AuthorizePaymentCommand) Handle(
	service *services.PaymentService,
) (*domain.Payment, error) {
	return service.AuthorizePayment(
		context.Background(),
		c.PaymentID,
	)
}

// AuthorizePaymentCommandFactory creates an AuthorizePaymentCommand.
func AuthorizePaymentCommandFactory(paymentID string) *AuthorizePaymentCommand {
	return &AuthorizePaymentCommand{
		PaymentID: paymentID,
	}
}