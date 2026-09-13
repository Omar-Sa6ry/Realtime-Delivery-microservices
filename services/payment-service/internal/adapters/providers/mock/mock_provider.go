package mock

import (
	"context"
	"fmt"
	"sync"

	"github.com/realtime-delivery/payment-service/internal/adapters/providers"
)

// MockProvider is a mock implementation of PaymentProvider for testing.
type MockProvider struct {
	mu                sync.RWMutex
	authorizeResults  map[string]*providers.ProviderResult
	authorizeErrors   map[string]*providers.NormalizedError
	captureResults    map[string]*providers.ProviderResult
	captureErrors     map[string]*providers.NormalizedError
	voidResults       map[string]*providers.ProviderResult
	voidErrors        map[string]*providers.NormalizedError
	refundResults     map[string]*providers.ProviderResult
	refundErrors      map[string]*providers.NormalizedError
	statusResults     map[string]*providers.ProviderResult
	statusErrors      map[string]*providers.NormalizedError
}

// NewMockProvider creates a new MockProvider.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		authorizeResults: make(map[string]*providers.ProviderResult),
		authorizeErrors:  make(map[string]*providers.NormalizedError),
		captureResults:   make(map[string]*providers.ProviderResult),
		captureErrors:    make(map[string]*providers.NormalizedError),
		voidResults:      make(map[string]*providers.ProviderResult),
		voidErrors:       make(map[string]*providers.NormalizedError),
		refundResults:    make(map[string]*providers.ProviderResult),
		refundErrors:     make(map[string]*providers.NormalizedError),
		statusResults:    make(map[string]*providers.ProviderResult),
		statusErrors:     make(map[string]*providers.NormalizedError),
	}
}

// SetAuthorizeResult sets the result for Authorize.
func (m *MockProvider) SetAuthorizeResult(paymentID string, result *providers.ProviderResult, err *providers.NormalizedError) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authorizeResults[paymentID] = result
	m.authorizeErrors[paymentID] = err
}

// SetCaptureResult sets the result for Capture.
func (m *MockProvider) SetCaptureResult(providerPaymentID string, result *providers.ProviderResult, err *providers.NormalizedError) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.captureResults[paymentID] = result
	m.captureErrors[paymentID] = err
}

// SetVoidResult sets the result for Void.
func (m *MockProvider) SetVoidResult(providerPaymentID string, result *providers.ProviderResult, err *providers.NormalizedError) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.voidResults[paymentID] = result
	m.voidErrors[paymentID] = err
}

// SetRefundResult sets the result for Refund.
func (m *MockProvider) SetRefundResult(providerPaymentID string, result *providers.ProviderResult, err *providers.NormalizedError) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refundResults[paymentID] = result
	m.refundErrors[paymentID] = err
}

// SetStatusResult sets the result for GetStatus.
func (m *MockProvider) SetStatusResult(providerPaymentID string, result *providers.ProviderResult, err *providers.NormalizedError) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusResults[paymentID] = result
	m.statusErrors[paymentID] = err
}

// Authorize authorizes a payment.
func (m *MockProvider) Authorize(ctx context.Context, req providers.AuthorizeRequest) (*providers.ProviderResult, *providers.NormalizedError) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err, ok := m.authorizeErrors[req.PaymentID]; ok && err != nil {
		return nil, err
	}

	if result, ok := m.authorizeResults[req.PaymentID]; ok {
		return result, nil
	}

	// Default success response
	return &providers.ProviderResult{
		ProviderTransactionID: fmt.Sprintf("pi_mock_%s", req.PaymentID),
		ProviderPaymentID:     fmt.Sprintf("pi_mock_%s", req.PaymentID),
		Status:                "AUTHORIZED",
		ClientSecret:          fmt.Sprintf("pi_mock_%s_secret_mock", req.PaymentID),
	}, nil
}

// Capture captures a payment.
func (m *MockProvider) Capture(ctx context.Context, req providers.CaptureRequest) (*providers.ProviderResult, *providers.NormalizedError) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err, ok := m.captureErrors[req.ProviderPaymentID]; ok && err != nil {
		return nil, err
	}

	if result, ok := m.captureResults[req.ProviderPaymentID]; ok {
		return result, nil
	}

	return &providers.ProviderResult{
		ProviderTransactionID: fmt.Sprintf("ch_mock_%s", req.ProviderPaymentID),
		ProviderPaymentID:     req.ProviderPaymentID,
		Status:                "SUCCEEDED",
	}, nil
}

// Void cancels an authorization.
func (m *MockProvider) Void(ctx context.Context, req providers.VoidRequest) (*providers.ProviderResult, *providers.NormalizedError) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err, ok := m.voidErrors[req.ProviderPaymentID]; ok && err != nil {
		return nil, err
	}

	if result, ok := m.voidResults[req.ProviderPaymentID]; ok {
		return result, nil
	}

	return &providers.ProviderResult{
		ProviderTransactionID: fmt.Sprintf("pi_mock_%s_cancel", req.ProviderPaymentID),
		ProviderPaymentID:     req.ProviderPaymentID,
		Status:                "CANCELLED",
	}, nil
}

// Refund refunds a payment.
func (m *MockProvider) Refund(ctx context.Context, req providers.RefundRequest) (*providers.ProviderResult, *providers.NormalizedError) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err, ok := m.refundErrors[req.ProviderPaymentID]; ok && err != nil {
		return nil, err
	}

	if result, ok := m.refundResults[req.ProviderPaymentID]; ok {
		return result, nil
	}

	return &providers.ProviderResult{
		ProviderTransactionID: fmt.Sprintf("re_mock_%s", req.ProviderPaymentID),
		ProviderPaymentID:     req.ProviderPaymentID,
		Status:                "SUCCEEDED",
	}, nil
}

// GetStatus retrieves the status of a payment.
func (m *MockProvider) GetStatus(ctx context.Context, providerPaymentID string) (*providers.ProviderResult, *providers.NormalizedError) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err, ok := m.statusErrors[providerPaymentID]; ok && err != nil {
		return nil, err
	}

	if result, ok := m.statusResults[providerPaymentID]; ok {
		return result, nil
	}

	// Default: return SUCCEEDED
	return &providers.ProviderResult{
		ProviderTransactionID: fmt.Sprintf("ch_mock_%s", providerPaymentID),
		ProviderPaymentID:     providerPaymentID,
		Status:                "SUCCEEDED",
	}, nil
}