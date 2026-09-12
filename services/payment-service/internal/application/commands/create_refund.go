package commands

import (
	"context"

	"github.com/realtime-delivery/payment-service/internal/application/services"
	"github.com/realtime-delivery/payment-service/internal/domain"
)

// CreateRefundCommand represents the command to create a refund.
type CreateRefundCommand struct {
	PaymentID   string
	Reason      string
	Amount      int64
	CorrelationID string
}

// Handle executes the create refund command.
func (c *CreateRefundCommand) Handle(
	service *services.PaymentService,
) (*domain.Refund, error) {
	return service.CreateRefund(
		context.Background(),
		c.PaymentID,
		c.Reason,
		c.Amount,
	)
}

// CreateRefundCommandFactory creates a CreateRefundCommand.
func CreateRefundCommandFactory(paymentID, reason string, amount int64, correlationID string) *CreateRefundCommand {
	return &CreateRefundCommand{
		PaymentID:   paymentID,
		Reason:      reason,
		Amount:      amount,
		CorrelationID: correlationID,
	}
}