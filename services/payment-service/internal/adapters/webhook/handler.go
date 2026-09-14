package webhook

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	stripego "github.com/stripe/stripe-go/v78"
	stripewh "github.com/stripe/stripe-go/v78/webhook"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/postgres"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

type Handler struct {
	webhookSecret string
	paymentRepo   *postgres.PaymentRepository
	outboxRepo    *postgres.OutboxRepository
}

func NewHandler(
	webhookSecret string,
	paymentRepo *postgres.PaymentRepository,
	outboxRepo *postgres.OutboxRepository,
) *Handler {
	return &Handler{
		webhookSecret: webhookSecret,
		paymentRepo:   paymentRepo,
		outboxRepo:    outboxRepo,
	}
}

func (h *Handler) Handle(c *gin.Context) {
	// 1. Read body — must happen before any parsing.
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		slog.Error("webhook: failed to read body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	// 2. Verify Stripe signature (HMAC-SHA256 + timestamp replay protection).
	sig := c.GetHeader("Stripe-Signature")
	event, err := stripewh.ConstructEvent(body, sig, h.webhookSecret)
	if err != nil {
		slog.Warn("webhook: invalid stripe signature", "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	ctx := c.Request.Context()

	// 3. Deduplicate — insert into processed_provider_events.
	dedupErr := h.outboxRepo.InsertProcessedProviderEvent(ctx, "stripe", event.ID, string(event.Type))
	if dedupErr != nil {
		if errors.Is(dedupErr, domain.ErrDuplicateIdempotency) {
			// Already processed — idempotent success.
			slog.Debug("webhook: duplicate event, skipping", "eventID", event.ID, "type", event.Type)
			c.JSON(http.StatusOK, gin.H{"received": true, "duplicate": true})
			return
		}
		slog.Error("webhook: dedup check failed", "eventID", event.ID, "error", dedupErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// 4. Process the event.
	slog.Info("webhook: processing event", "eventID", event.ID, "type", event.Type)
	if err := h.processEvent(c, event); err != nil {
		slog.Error("webhook: processing failed", "eventID", event.ID, "type", event.Type, "error", err)
		c.JSON(http.StatusOK, gin.H{"received": true, "processed": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"received": true, "processed": true})
}

func (h *Handler) processEvent(c *gin.Context, event stripego.Event) error {
	ctx := c.Request.Context()
	switch event.Type {
	case "payment_intent.succeeded":
		return h.handlePaymentIntentSucceeded(ctx, event)
	case "payment_intent.payment_failed":
		return h.handlePaymentIntentFailed(ctx, event)
	case "payment_intent.canceled":
		return h.handlePaymentIntentCanceled(ctx, event)
	case "charge.refunded":
		return h.handleChargeRefunded(ctx, event)
	case "payment_intent.requires_action":
		slog.Info("webhook: payment requires customer action", "eventID", event.ID)
		return nil
	default:
		slog.Debug("webhook: unhandled event type", "type", event.Type)
		return nil
	}
}

func (h *Handler) handlePaymentIntentSucceeded(ctx interface{ Done() <-chan struct{} }, event stripego.Event) error {
	var pi stripego.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return err
	}

	c, ok := ctx.(interface{ Request() *http.Request })
	_ = ok
	_ = c

	return nil
}

func (h *Handler) handlePaymentIntentFailed(ctx interface{ Done() <-chan struct{} }, event stripego.Event) error {
	var pi stripego.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return err
	}
	return nil 
}

func (h *Handler) handlePaymentIntentCanceled(ctx interface{ Done() <-chan struct{} }, event stripego.Event) error {
	var pi stripego.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return err
	}
	return nil 
}

func (h *Handler) handleChargeRefunded(ctx interface{ Done() <-chan struct{} }, event stripego.Event) error {
	var ch stripego.Charge
	if err := json.Unmarshal(event.Data.Raw, &ch); err != nil {
		return err
	}
	return nil 
}
