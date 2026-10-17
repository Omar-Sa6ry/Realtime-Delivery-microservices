package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	pkgevents "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	pkgsnowflake "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/snowflake"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/nats"
	pgadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/postgres"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/observability"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/ports"
)

const paymentMethod = "card"

type PaymentService struct {
	paymentRepo  *pgadapter.PaymentRepository
	refundRepo   *pgadapter.RefundRepository
	attemptRepo  *pgadapter.AttemptRepository
	outboxRepo   *pgadapter.OutboxRepository
	provider     ports.PaymentProvider
	nats         *nats.RealtimePublisher // may be nil if NATS is unavailable
	snowflake    *pkgsnowflake.Snowflake
}

func NewPaymentService(
	paymentRepo *pgadapter.PaymentRepository,
	refundRepo *pgadapter.RefundRepository,
	attemptRepo *pgadapter.AttemptRepository,
	outboxRepo *pgadapter.OutboxRepository,
	provider ports.PaymentProvider,
	nats *nats.RealtimePublisher,
	snowflake *pkgsnowflake.Snowflake,
) *PaymentService {
	return &PaymentService{
		paymentRepo: paymentRepo,
		refundRepo:  refundRepo,
		attemptRepo: attemptRepo,
		outboxRepo:  outboxRepo,
		provider:    provider,
		nats:        nats,
		snowflake:   snowflake,
	}
}

type CreatePaymentInput struct {
	DeliveryID     string
	UserID         string
	AmountMinor    int64
	Currency       string
	IdempotencyKey string
	CorrelationID  string
	CausationID    string
}

type CreatePaymentResult struct {
	PaymentID    string
	Status       domain.PaymentStatus
	ClientSecret string // Stripe client_secret for frontend confirmation
	CheckoutURL  string
}

func (s *PaymentService) CreatePayment(ctx context.Context, input CreatePaymentInput) (result *CreatePaymentResult, err error) {
	// Generate IDs.
	paymentID := strconv.FormatInt(s.snowflake.NextID(), 10)
	attemptID := strconv.FormatInt(s.snowflake.NextID(), 10)

	op := observability.StartOperation(ctx, "create", paymentID)
	ctx = op.Context()
	op.AddAttribute("delivery.id", input.DeliveryID)
	op.AddAttribute("user.id", input.UserID)
	op.AddAttribute("amount_minor", input.AmountMinor)
	op.AddAttribute("currency", input.Currency)
	defer func() { op.Finish(err) }()

	payment, err := domain.NewPayment(
		paymentID, input.DeliveryID, input.UserID,
		input.AmountMinor, input.Currency,
		input.CorrelationID, input.CausationID,
	)
	if err != nil {
		return nil, fmt.Errorf("create_payment: invalid input: %w", err)
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("create_payment: persist: %w", err)
	}
	observability.M().RecordPaymentCreated(input.Currency, paymentMethod)

	if err := s.publishEvent(ctx, pkgevents.PaymentCreated, paymentID, pkgevents.PaymentCreatedPayload{
		PaymentID:     paymentID,
		DeliveryID:    input.DeliveryID,
		UserID:        input.UserID,
		AmountMinor:   input.AmountMinor,
		Currency:      input.Currency,
		Status:        string(payment.Status),
		CorrelationID: input.CorrelationID,
		CausationID:   input.CausationID,
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		slog.Warn("create_payment: outbox publish failed (non-fatal)", "error", err)
	}

	attempt := domain.NewAttempt(attemptID, paymentID, "AUTHORIZE", "stripe", input.IdempotencyKey+"-auth", 1)
	_ = s.attemptRepo.Create(ctx, attempt)

	authResult, authErr := s.provider.Authorize(ctx, ports.AuthorizeRequest{
		PaymentID:      paymentID,
		AmountMinor:    input.AmountMinor,
		Currency:       input.Currency,
		IdempotencyKey: input.IdempotencyKey,
		Description:    fmt.Sprintf("Delivery %s", input.DeliveryID),
	})

	if authErr != nil {
		if authErr.Category == ports.ErrCategoryTimeout {
			attempt.MarkUnknown()
			_ = s.attemptRepo.UpdateStatus(ctx, attemptID, domain.OperationStatusUnknown, "")
			slog.Warn("create_payment: provider timeout â€” reconciliation required", "paymentID", paymentID)
			return &CreatePaymentResult{
				PaymentID: paymentID,
				Status:    payment.Status,
			}, domain.ErrProviderUnknownOutcome
		}

		// Permanent failure.
		attempt.Fail(string(authErr.Category), string(authErr.Category), authErr.Message)
		_ = s.attemptRepo.UpdateStatus(ctx, attemptID, domain.OperationStatusFailed, "")
		_ = payment.Fail()
		_ = s.paymentRepo.UpdateConditional(ctx, payment, 0)
		observability.M().RecordPaymentFailed(input.Currency, paymentMethod, string(authErr.Category))
		return nil, fmt.Errorf("create_payment: authorization failed: %s", authErr.Message)
	}

	// Update payment to AUTHORIZED.
	if err := payment.Authorize(authResult.ProviderPaymentID, authResult.ClientSecret, input.AmountMinor); err != nil {
		return nil, fmt.Errorf("create_payment: state transition: %w", err)
	}

	attempt.Succeed(authResult.ProviderTransactionID)
	_ = s.attemptRepo.UpdateStatus(ctx, attemptID, domain.OperationStatusSucceeded, authResult.ProviderTransactionID)

	if err := s.paymentRepo.UpdateConditional(ctx, payment, 0); err != nil {
		return nil, fmt.Errorf("create_payment: update payment: %w", err)
	}
	observability.M().RecordPaymentAuthorized(input.Currency, paymentMethod)

	// Publish authorized event.
	now := time.Now().UTC()
	_ = s.publishEvent(ctx, pkgevents.PaymentAuthorized, paymentID, pkgevents.PaymentAuthorizedPayload{
		PaymentID:             paymentID,
		DeliveryID:            input.DeliveryID,
		UserID:                input.UserID,
		AmountMinor:           input.AmountMinor,
		Currency:              input.Currency,
		AuthorizedAmountMinor: payment.AuthorizedAmountMinor,
		ProviderPaymentID:     authResult.ProviderPaymentID,
		CorrelationID:         input.CorrelationID,
		CausationID:           paymentID,
		AuthorizedAt:          now,
	})

	// NATS realtime notification (fire-and-forget).
	s.publishRealtime(ctx, paymentID, input.DeliveryID, input.UserID, string(payment.Status))

	slog.Info("create_payment: success",
		"paymentID", paymentID,
		"status", payment.Status,
		"provider", authResult.ProviderPaymentID,
	)

	return &CreatePaymentResult{
		PaymentID:    paymentID,
		Status:       payment.Status,
		ClientSecret: authResult.ClientSecret,
		CheckoutURL:  authResult.CheckoutURL,
	}, nil
}

type CapturePaymentInput struct {
	PaymentID      string
	AmountMinor    int64
	IdempotencyKey string
	CorrelationID  string
}

func (s *PaymentService) CapturePayment(ctx context.Context, input CapturePaymentInput) (payment *domain.Payment, err error) {
	op := observability.StartOperation(ctx, "capture", input.PaymentID)
	ctx = op.Context()
	op.AddAttribute("amount_minor", input.AmountMinor)
	defer func() { op.Finish(err) }()

	payment, err = s.paymentRepo.FindByID(ctx, input.PaymentID)
	if err != nil {
		return nil, err
	}

	if !payment.CanCapture() {
		return nil, fmt.Errorf("%w: status=%s", domain.ErrPaymentNotCaptureable, payment.Status)
	}

	attemptID := strconv.FormatInt(s.snowflake.NextID(), 10)
	attempt := domain.NewAttempt(attemptID, payment.ID, "CAPTURE", "stripe", input.IdempotencyKey+"-cap", 1)
	_ = s.attemptRepo.Create(ctx, attempt)

	captureResult, captureErr := s.provider.Capture(ctx, ports.CaptureRequest{
		ProviderPaymentID: payment.ProviderPaymentID,
		AmountMinor:       input.AmountMinor,
		IdempotencyKey:    input.IdempotencyKey,
	})

	if captureErr != nil {
		if captureErr.Category == ports.ErrCategoryTimeout {
			attempt.MarkUnknown()
			_ = s.attemptRepo.UpdateStatus(ctx, attemptID, domain.OperationStatusUnknown, "")
			return nil, domain.ErrProviderUnknownOutcome
		}
		attempt.Fail(string(captureErr.Category), string(captureErr.Category), captureErr.Message)
		_ = s.attemptRepo.UpdateStatus(ctx, attemptID, domain.OperationStatusFailed, "")
		observability.M().RecordPaymentFailed(payment.Currency, paymentMethod, string(captureErr.Category))
		return nil, fmt.Errorf("capture failed: %s", captureErr.Message)
	}

	prevVersion := payment.Version
	if err := payment.Capture(input.AmountMinor); err != nil {
		return nil, err
	}

	attempt.Succeed(captureResult.ProviderTransactionID)
	_ = s.attemptRepo.UpdateStatus(ctx, attemptID, domain.OperationStatusSucceeded, captureResult.ProviderTransactionID)

	if err := s.paymentRepo.UpdateConditional(ctx, payment, prevVersion); err != nil {
		return nil, err
	}
	observability.M().RecordPaymentCaptured(payment.Currency, paymentMethod)

	now := time.Now().UTC()
	_ = s.publishEvent(ctx, pkgevents.PaymentCaptured, payment.ID, pkgevents.PaymentCapturedPayload{
		PaymentID:             payment.ID,
		DeliveryID:            payment.DeliveryID,
		UserID:                payment.UserID,
		AmountMinor:           payment.AmountMinor,
		Currency:              payment.Currency,
		CapturedAmountMinor:   payment.CapturedAmountMinor,
		ProviderTransactionID: captureResult.ProviderTransactionID,
		CorrelationID:         input.CorrelationID,
		CausationID:           input.PaymentID,
		CapturedAt:            now,
	})
	// Emit canonical payment.completed for inter-service consumers (delivery, notification, analytics)
	_ = s.publishEvent(ctx, pkgevents.PaymentCompleted, payment.ID, pkgevents.PaymentCapturedPayload{
		PaymentID:             payment.ID,
		DeliveryID:            payment.DeliveryID,
		UserID:                payment.UserID,
		AmountMinor:           payment.AmountMinor,
		Currency:              payment.Currency,
		CapturedAmountMinor:   payment.CapturedAmountMinor,
		ProviderTransactionID: captureResult.ProviderTransactionID,
		CorrelationID:         input.CorrelationID,
		CausationID:           input.PaymentID,
		CapturedAt:            now,
	})
	s.publishRealtime(ctx, payment.ID, payment.DeliveryID, payment.UserID, string(payment.Status))

	return payment, nil
}

type AuthorizePaymentInput struct {
	PaymentID      string
	IdempotencyKey string
	CorrelationID  string
}

func (s *PaymentService) AuthorizePayment(ctx context.Context, input AuthorizePaymentInput) (payment *domain.Payment, err error) {
	op := observability.StartOperation(ctx, "authorize", input.PaymentID)
	ctx = op.Context()
	defer func() { op.Finish(err) }()

	payment, err = s.paymentRepo.FindByID(ctx, input.PaymentID)
	if err != nil {
		return nil, err
	}
	if !payment.CanAuthorize() {
		return nil, fmt.Errorf("%w: status=%s", domain.ErrPaymentInvalidStatus, payment.Status)
	}

	attemptID := strconv.FormatInt(s.snowflake.NextID(), 10)
	attempt := domain.NewAttempt(attemptID, payment.ID, "AUTHORIZE", "stripe", input.IdempotencyKey+"-auth", 1)
	_ = s.attemptRepo.Create(ctx, attempt)

	authResult, authErr := s.provider.Authorize(ctx, ports.AuthorizeRequest{
		PaymentID:      payment.ID,
		AmountMinor:    payment.AmountMinor,
		Currency:       payment.Currency,
		IdempotencyKey: input.IdempotencyKey,
		Description:    fmt.Sprintf("Delivery %s", payment.DeliveryID),
	})
	if authErr != nil {
		if authErr.Category == ports.ErrCategoryTimeout {
			attempt.MarkUnknown()
			_ = s.attemptRepo.UpdateStatus(ctx, attemptID, domain.OperationStatusUnknown, "")
			return nil, domain.ErrProviderUnknownOutcome
		}
		attempt.Fail(string(authErr.Category), string(authErr.Category), authErr.Message)
		_ = s.attemptRepo.UpdateStatus(ctx, attemptID, domain.OperationStatusFailed, "")
		observability.M().RecordPaymentFailed(payment.Currency, paymentMethod, string(authErr.Category))
		return nil, fmt.Errorf("authorization failed: %s", authErr.Message)
	}

	prevVersion := payment.Version
	if err := payment.Authorize(authResult.ProviderPaymentID, authResult.ClientSecret, payment.AmountMinor); err != nil {
		return nil, err
	}

	attempt.Succeed(authResult.ProviderTransactionID)
	_ = s.attemptRepo.UpdateStatus(ctx, attemptID, domain.OperationStatusSucceeded, authResult.ProviderTransactionID)

	if err := s.paymentRepo.UpdateConditional(ctx, payment, prevVersion); err != nil {
		return nil, err
	}
	observability.M().RecordPaymentAuthorized(payment.Currency, paymentMethod)

	now := time.Now().UTC()
	_ = s.publishEvent(ctx, pkgevents.PaymentAuthorized, payment.ID, pkgevents.PaymentAuthorizedPayload{
		PaymentID:             payment.ID,
		DeliveryID:            payment.DeliveryID,
		UserID:                payment.UserID,
		AmountMinor:           payment.AmountMinor,
		Currency:              payment.Currency,
		AuthorizedAmountMinor: payment.AuthorizedAmountMinor,
		ProviderPaymentID:     authResult.ProviderPaymentID,
		CorrelationID:         input.CorrelationID,
		CausationID:           input.PaymentID,
		AuthorizedAt:          now,
	})
	s.publishRealtime(ctx, payment.ID, payment.DeliveryID, payment.UserID, string(payment.Status))

	return payment, nil
}

func (s *PaymentService) CancelAuthorization(ctx context.Context, paymentID, idempotencyKey string) (payment *domain.Payment, err error) {
	op := observability.StartOperation(ctx, "cancel", paymentID)
	ctx = op.Context()
	defer func() { op.Finish(err) }()

	payment, err = s.paymentRepo.FindByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	if !payment.CanCancel() {
		return nil, fmt.Errorf("%w: status=%s", domain.ErrPaymentInvalidStatus, payment.Status)
	}

	attemptID := strconv.FormatInt(s.snowflake.NextID(), 10)
	attempt := domain.NewAttempt(attemptID, payment.ID, "CANCEL", "stripe", idempotencyKey+"-void", 1)
	_ = s.attemptRepo.Create(ctx, attempt)

	voidResult, voidErr := s.provider.Void(ctx, ports.VoidRequest{
		ProviderPaymentID: payment.ProviderPaymentID,
		IdempotencyKey:    idempotencyKey,
	})
	if voidErr != nil {
		if voidErr.Category == ports.ErrCategoryTimeout {
			attempt.MarkUnknown()
			_ = s.attemptRepo.UpdateStatus(ctx, attemptID, domain.OperationStatusUnknown, "")
			return nil, domain.ErrProviderUnknownOutcome
		}
		attempt.Fail(string(voidErr.Category), string(voidErr.Category), voidErr.Message)
		_ = s.attemptRepo.UpdateStatus(ctx, attemptID, domain.OperationStatusFailed, "")
		observability.M().RecordPaymentFailed(payment.Currency, paymentMethod, string(voidErr.Category))
		return nil, fmt.Errorf("void failed: %s", voidErr.Message)
	}

	prevVersion := payment.Version
	if err := payment.Cancel(); err != nil {
		return nil, err
	}

	attempt.Succeed(voidResult.ProviderTransactionID)
	_ = s.attemptRepo.UpdateStatus(ctx, attemptID, domain.OperationStatusSucceeded, voidResult.ProviderTransactionID)

	if err := s.paymentRepo.UpdateConditional(ctx, payment, prevVersion); err != nil {
		return nil, err
	}
	observability.M().RecordPaymentCancelled(payment.Currency, paymentMethod)

	now := time.Now().UTC()
	_ = s.publishEvent(ctx, pkgevents.PaymentCancelled, payment.ID, pkgevents.PaymentCancelledPayload{
		PaymentID:   payment.ID,
		DeliveryID:  payment.DeliveryID,
		UserID:      payment.UserID,
		CancelledAt: now,
	})
	s.publishRealtime(ctx, payment.ID, payment.DeliveryID, payment.UserID, string(payment.Status))

	return payment, nil
}

type CreateRefundInput struct {
	PaymentID      string
	AmountMinor    int64
	Reason         string
	IdempotencyKey string
	CorrelationID  string
}

func (s *PaymentService) CreateRefund(ctx context.Context, input CreateRefundInput) (refund *domain.Refund, err error) {
	op := observability.StartOperation(ctx, "refund", input.PaymentID)
	ctx = op.Context()
	op.AddAttribute("amount_minor", input.AmountMinor)
	op.AddAttribute("reason", input.Reason)
	defer func() { op.Finish(err) }()

	payment, err := s.paymentRepo.FindByID(ctx, input.PaymentID)
	if err != nil {
		return nil, err
	}

	prevVersion := payment.Version
	if err := payment.ReserveRefund(input.AmountMinor); err != nil {
		return nil, err
	}
	if err := s.paymentRepo.UpdateConditional(ctx, payment, prevVersion); err != nil {
		return nil, fmt.Errorf("create_refund: reserve refund concurrency conflict: %w", err)
	}

	refundID := strconv.FormatInt(s.snowflake.NextID(), 10)
	refund, err = domain.NewRefund(
		refundID, payment.ID, payment.DeliveryID,
		input.AmountMinor, payment.Currency, input.Reason,
		input.IdempotencyKey, input.CorrelationID, input.PaymentID,
	)
	if err != nil {
		_ = payment.ReleaseRefundReservation(input.AmountMinor)
		_ = s.paymentRepo.UpdateConditional(ctx, payment, payment.Version-1)
		return nil, err
	}

	if err := s.refundRepo.Create(ctx, refund); err != nil {
		_ = payment.ReleaseRefundReservation(input.AmountMinor)
		_ = s.paymentRepo.UpdateConditional(ctx, payment, payment.Version-1)
		return nil, fmt.Errorf("create_refund: persist refund: %w", err)
	}

	refResult, refErr := s.provider.Refund(ctx, ports.RefundRequest{
		ProviderPaymentID: payment.ProviderPaymentID,
		AmountMinor:       input.AmountMinor,
		Reason:            input.Reason,
		IdempotencyKey:    input.IdempotencyKey,
	})

	if refErr != nil {
		refund.Fail(refErr.Message)
		_ = s.refundRepo.UpdateStatus(ctx, refund.ID, domain.RefundStatusFailed, "")
		_ = payment.ReleaseRefundReservation(input.AmountMinor)
		_ = s.paymentRepo.UpdateConditional(ctx, payment, payment.Version-1)
		observability.M().RecordPaymentFailed(payment.Currency, paymentMethod, "provider_error")
		return nil, fmt.Errorf("refund failed: %s", refErr.Message)
	}

	refund.Complete(refResult.ProviderTransactionID)
	_ = s.refundRepo.UpdateStatus(ctx, refund.ID, domain.RefundStatusCompleted, refResult.ProviderTransactionID)

	prevVersion = payment.Version
	_ = payment.CommitRefund(input.AmountMinor)
	_ = s.paymentRepo.UpdateConditional(ctx, payment, prevVersion)
	observability.M().RecordPaymentRefunded(payment.Currency, paymentMethod)

	now := time.Now().UTC()
	_ = s.publishEvent(ctx, pkgevents.PaymentRefunded, payment.ID, pkgevents.PaymentRefundedPayload{
		PaymentID:           payment.ID,
		DeliveryID:          payment.DeliveryID,
		UserID:              payment.UserID,
		AmountMinor:         payment.AmountMinor,
		RefundedAmountMinor: input.AmountMinor,
		Currency:            payment.Currency,
		ProviderRefundID:    refResult.ProviderTransactionID,
		CorrelationID:       input.CorrelationID,
		CausationID:         input.PaymentID,
		RefundedAt:          now,
	})
	s.publishRealtime(ctx, payment.ID, payment.DeliveryID, payment.UserID, string(payment.Status))

	return refund, nil
}

func (s *PaymentService) GetPayment(ctx context.Context, paymentID string) (*domain.Payment, error) {
	return s.paymentRepo.FindByID(ctx, paymentID)
}

func (s *PaymentService) GetPaymentStatus(ctx context.Context, paymentID string) (domain.PaymentStatus, error) {
	p, err := s.paymentRepo.FindByID(ctx, paymentID)
	if err != nil {
		return "", err
	}
	return p.Status, nil
}

// ListPayments retrieves payments with pagination and optional user filter.
func (s *PaymentService) ListPayments(ctx context.Context, page, limit int, userID string) ([]*domain.Payment, int, error) {
	return s.paymentRepo.List(ctx, page, limit, userID)
}

func (s *PaymentService) publishEvent(ctx context.Context, eventType pkgevents.PaymentEventType, aggregateID string, payload interface{}) error {
	eventID := strconv.FormatInt(s.snowflake.NextID(), 10)
	data, err := pkgevents.MarshalPaymentEnvelope(eventID, eventType, "", payload)
	if err != nil {
		return fmt.Errorf("publishEvent: marshal: %w", err)
	}
	return s.outboxRepo.Insert(ctx, string(eventType), data)
}

func (s *PaymentService) publishRealtime(ctx context.Context, paymentID, deliveryID, userID, status string) {
	if s.nats == nil {
		return
	}
	payload := nats.PaymentStatusUpdate{
		PaymentID:  paymentID,
		DeliveryID: deliveryID,
		UserID:     userID,
		Status:     status,
		UpdatedAt:  time.Now().UTC(),
	}
	data, _ := json.Marshal(payload)
	_ = s.nats.PublishPaymentStatusUpdated(ctx, data)
}

func (s *PaymentService) PaymentRepository() ports.PaymentRepository {
	return s.paymentRepo
}
