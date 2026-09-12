package commands

import (
	"context"

	"github.com/realtime-delivery/payment-service/internal/application/services"
	"github.com/realtime-delivery/payment-service/internal/domain"
	"github.com/realtime-delivery/payment-service/internal/ports"
)

// CreatePaymentCommand represents the command to create a new payment.
type CreatePaymentCommand struct {
	DeliveryID  string
	UserID      string
	Amount      int64
	Currency    string
	CorrelationID string
	CausationID   string
}

// Handle executes the create payment command.
func (c *CreatePaymentCommand) Handle(
	service *services.PaymentService,
) (*domain.Payment, error) {
	return service.CreatePayment(
		context.Background(),
		c.DeliveryID,
		c.UserID,
		c.Amount,
		c.Currency,
	)
}

// CreatePaymentCommandFactory creates a CreatePaymentCommand from inputs.
func CreatePaymentCommandFactory(deliveryID, userID string, amount int64, currency string, correlationID, causationID string) *CreatePaymentCommand {
	return &CreatePaymentCommand{
		DeliveryID:  deliveryID,
		UserID:      userID,
		Amount:      amount,
		Currency:    currency,
		CorrelationID: correlationID,
		CausationID: causationID,
	}
}