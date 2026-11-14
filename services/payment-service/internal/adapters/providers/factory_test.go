package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/providers/stripe"
)

func TestNewProviderFactoryKeepsSecretKey(t *testing.T) {
	f := NewProviderFactory("sk_test_123")
	assert.Equal(t, "sk_test_123", f.stripeSecretKey)
}

func TestCreateProviderStripeByName(t *testing.T) {
	f := NewProviderFactory("sk_test_abc")

	for _, name := range []string{"stripe", ""} {
		p, err := f.CreateProvider(name)
		require.NoError(t, err, name)
		require.NotNil(t, p, name)
		_, ok := p.(*stripe.StripeProvider)
		assert.True(t, ok, name)
	}
}

func TestCreateProviderUnsupported(t *testing.T) {
	f := NewProviderFactory("sk_test_abc")

	p, err := f.CreateProvider("paypal")
	require.Error(t, err)
	assert.Nil(t, p)
	assert.ErrorContains(t, err, "unsupported payment provider: paypal")
}
