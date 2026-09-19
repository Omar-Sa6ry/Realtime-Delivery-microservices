package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	pkgevents "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
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
	event, err := stripewh.ConstructEventWithOptions(body, sig, h.webhookSecret, stripewh.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
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
	case "checkout.session.completed":
		return h.handleCheckoutSessionCompleted(ctx, event)
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

func (h *Handler) handleCheckoutSessionCompleted(ctx context.Context, event stripego.Event) error {
	var sess stripego.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return err
	}

	paymentID := ""
	if sess.Metadata != nil {
		paymentID = sess.Metadata["payment_id"]
	}

	var p *domain.Payment
	var err error
	if paymentID != "" {
		p, err = h.paymentRepo.FindByID(ctx, paymentID)
	}
	if p == nil && sess.PaymentIntent != nil {
		p, err = h.paymentRepo.FindByProviderPaymentID(ctx, sess.PaymentIntent.ID)
	}

	if err != nil || p == nil {
		slog.Warn("webhook: checkout session payment not found", "sessionID", sess.ID, "paymentID", paymentID, "error", err)
		return nil
	}

	return h.completePayment(ctx, p, sess.ID)
}

func (h *Handler) handlePaymentIntentSucceeded(ctx context.Context, event stripego.Event) error {
	var pi stripego.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return err
	}

	paymentID := ""
	if pi.Metadata != nil {
		paymentID = pi.Metadata["payment_id"]
	}

	var p *domain.Payment
	var err error
	if paymentID != "" {
		p, err = h.paymentRepo.FindByID(ctx, paymentID)
	}
	if p == nil {
		p, err = h.paymentRepo.FindByProviderPaymentID(ctx, pi.ID)
	}

	if err != nil || p == nil {
		slog.Warn("webhook: payment intent not found", "piID", pi.ID, "paymentID", paymentID, "error", err)
		return nil
	}

	return h.completePayment(ctx, p, pi.ID)
}

func (h *Handler) completePayment(ctx context.Context, p *domain.Payment, providerTxID string) error {
	if p.Status == domain.PaymentStatusCaptured {
		slog.Info("webhook: payment already captured", "paymentID", p.ID, "status", p.Status)
		return nil
	}

	prevVersion := p.Version
	now := time.Now().UTC()
	p.Status = domain.PaymentStatusCaptured
	p.CapturedAmountMinor = p.AmountMinor
	p.AuthorizedAmountMinor = p.AmountMinor
	p.CapturedAt = &now
	p.AuthorizedAt = &now
	p.UpdatedAt = now
	p.Version++

	if err := h.paymentRepo.UpdateConditional(ctx, p, prevVersion); err != nil {
		slog.Error("webhook: failed to update payment", "paymentID", p.ID, "error", err)
		return err
	}

	// Insert PaymentCompleted event into outbox so delivery-service is notified!
	eventID := fmt.Sprintf("%d", time.Now().UnixNano())
	payload := pkgevents.PaymentCapturedPayload{
		PaymentID:             p.ID,
		DeliveryID:            p.DeliveryID,
		UserID:                p.UserID,
		AmountMinor:           p.AmountMinor,
		Currency:              p.Currency,
		CapturedAmountMinor:   p.CapturedAmountMinor,
		ProviderTransactionID: providerTxID,
		CorrelationID:         p.CorrelationID,
		CausationID:           p.ID,
		CapturedAt:            now,
	}

	data, err := pkgevents.MarshalPaymentEnvelope(eventID, pkgevents.PaymentCompleted, "", payload)
	if err == nil {
		_ = h.outboxRepo.Insert(ctx, string(pkgevents.PaymentCompleted), data)
		slog.Info("webhook: payment.completed published to outbox", "deliveryID", p.DeliveryID, "paymentID", p.ID)
	}

	return nil
}

func (h *Handler) handlePaymentIntentFailed(ctx context.Context, event stripego.Event) error {
	var pi stripego.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return err
	}
	return nil 
}

func (h *Handler) handlePaymentIntentCanceled(ctx context.Context, event stripego.Event) error {
	var pi stripego.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return err
	}
	return nil 
}

func (h *Handler) handleChargeRefunded(ctx context.Context, event stripego.Event) error {
	var ch stripego.Charge
	if err := json.Unmarshal(event.Data.Raw, &ch); err != nil {
		return err
	}
	return nil 
}
