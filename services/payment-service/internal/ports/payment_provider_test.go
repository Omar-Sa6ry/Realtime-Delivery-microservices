package ports_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/ports"
)

func TestNormalizedErrorError(t *testing.T) {
	e := &ports.NormalizedError{
		Category:  ports.ErrCategoryDeclined,
		Message:   "card declined",
		Retryable: false,
	}
	assert.Equal(t, "DECLINED: card declined", e.Error())
}

func TestErrorCategoryValues(t *testing.T) {
	// keep the public contract stable for consumers switching on these
	assert.Equal(t, ports.ErrorCategory("TEMPORARY"), ports.ErrCategoryTemporary)
	assert.Equal(t, ports.ErrorCategory("PERMANENT"), ports.ErrCategoryPermanent)
	assert.Equal(t, ports.ErrorCategory("DECLINED"), ports.ErrCategoryDeclined)
	assert.Equal(t, ports.ErrorCategory("AUTHENTICATION_ERROR"), ports.ErrCategoryAuthError)
	assert.Equal(t, ports.ErrorCategory("RATE_LIMITED"), ports.ErrCategoryRateLimited)
	assert.Equal(t, ports.ErrorCategory("TIMEOUT"), ports.ErrCategoryTimeout)
	assert.Equal(t, ports.ErrorCategory("UNKNOWN"), ports.ErrCategoryUnknown)
}
