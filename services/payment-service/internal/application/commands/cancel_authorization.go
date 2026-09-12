package commands

import (
	"context"

	"github.com/realtime-delivery/payment-service/internal/application/services"
	"github.com/realtime-delivery/payment-service/internal/domain"
)

type CancelAuthorizationCommand struct {
	PaymentID string
}

// Handle executes the cancel authorization command.
func (c *CancelAuthorizationCommand) Handle(
	service *services.PaymentService,
) (*domain.Payment, error) {
	return service.CancelAuthorization(
		context.Background(),
		c.PaymentID,
	)
}

// CancelAuthorizationCommandFactory creates a CancelAuthorizationCommand.
func CancelAuthorizationCommandFactory(paymentID string) *CancelAuthorizationCommand {
	return &CancelAuthorizationCommand{
		PaymentID: paymentID,
	}
}