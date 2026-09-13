package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/webhook"

	"github.com/realtime-delivery/payment-service/internal/domain"
	"github.com/realtime-delivery/payment-service/internal/ports"
)

// Handler handles Stripe webhook events.
type Handler struct {
	webhookSecret string
	paymentSvc   *services.PaymentService
}

// NewHandler creates a new webhook handler.
func NewHandler(webhookSecret string, paymentSvc *services.PaymentService) *Handler {
	return &Handler{
		webhookSecret: webhookSecret,
		paymentSvc:    paymentSvc,
	}
}

// Handle handles incoming Stripe webhook events.
func (h *Handler) Handle(c *gin.Context) {
	// Read the request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		// Log error
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Verify the Stripe signature
	signature := c.GetHeader("Stripe-Signature")
	event, err := webhook.ConstructEvent(body, c.GetHeader("Stripe-Signature"), webhookSecret)
	if err != nil {
		// Log error
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid signature"})
		return
	}

	// Check for duplicate events (idempotency)
	// This would check the processed_provider_events table
	// For now, we'll process the event

	// Process the event based on type
	err = h.processEvent(event)
	if err != nil {
		// Log error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

// processEvent processes a Stripe event.
func (h *Handler) processEvent(event stripe.Event) error {
	switch event.Type {
	case "payment_intent.succeeded":
		return h.handlePaymentIntentSucceeded(event)
	case "payment_intent.payment_failed":
		return h.handlePaymentIntentFailed(event)
	case "charge.refunded":
		return h.handleChargeRefunded(event)
	case "payment_intent.canceled":
		return h.handlePaymentIntentCanceled(event)
	case "payment_intent.processing":
		return h.handlePaymentIntentProcessing(event)
	case "payment_intent.requires_action":
		return h.handlePaymentIntentRequiresAction(event)
	default:
		// Log unhandled event type
		return nil
	}
}

func (h *Handler) handlePaymentIntentSucceeded(event stripe.Event) error {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return fmt.Errorf("failed to unmarshal payment_intent: %w", err)
	}

	// TODO: Process successful payment
	// 1. Find payment by provider_payment_id (pi.ID)
	// 2. Update payment status to CAPTURED (or AUTHORIZED if requires_capture)
	// 3. Create audit log
	// 4. Create outbox event
	// 5. Publish NATS realtime update

	return nil
}

func (h *Handler) handlePaymentIntentFailed(event stripe.Event) error {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return fmt.Errorf("failed to unmarshal payment_intent: %w", err)
	}

	// TODO: Process failed payment
	// 1. Find payment by provider_payment_id
	// 2. Update payment status to FAILED
	// 3. Create audit log
	// 4. Create outbox event with failure reason

	return nil
}

func (h *Handler) handleChargeRefunded(event stripe.Event) error {
	var ch stripe.Charge
	if err := json.Unmarshal(event.Data.Raw, &ch); err != nil {
		return fmt.Errorf("failed to unmarshal charge: %w", err)
	}

	// TODO: Process refund
	// 1. Find payment by provider_payment_id (ch.PaymentIntent)
	// 2. Update payment status to REFUNDED
	// 3. Create refund record
	// 4. Create audit log
	// 5. Create outbox event

	return nil
}

func (h *Handler) handlePaymentIntentCanceled(event stripe.Event) error {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return fmt.Errorf("failed to unmarshal payment_intent: %w", err)
	}

	// TODO: Process cancellation
	// 1. Find payment by provider_payment_id
	// 2. Update payment status to CANCELLED
	// 6. Create audit log
	// 7. Create outbox event

	return nil
}

func (h *Handler) handlePaymentIntentProcessing(event stripe.Event) error {
	// Payment is being processed - update status to PROCESSING
	return nil
}

func (h *Handler) handlePaymentIntentRequiresAction(event stripe.Event) error {
	// Payment requires 3D Secure or other action
	return nil
}

// WebhookEvent represents a processed webhook event for deduplication.
type WebhookEvent struct {
	ID        string
	Provider  string
	EventID   string
	EventType string
	ReceivedAt time.Time
	Processed bool
}