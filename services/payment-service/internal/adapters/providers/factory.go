package providers

import (
	"fmt"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/providers/stripe"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/ports"
)

type ProviderFactory struct {
	stripeSecretKey string
}

func NewProviderFactory(stripeSecretKey string) *ProviderFactory {
	return &ProviderFactory{
		stripeSecretKey: stripeSecretKey,
	}
}

func (f *ProviderFactory) CreateProvider(name string) (ports.PaymentProvider, error) {
	switch name {
	case "stripe", "":
		return stripe.NewStripeProvider(f.stripeSecretKey), nil
	default:
		return nil, fmt.Errorf("unsupported payment provider: %s", name)
	}
}
