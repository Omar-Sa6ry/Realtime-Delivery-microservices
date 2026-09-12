package postgres

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/realtime-delivery/payment-service/internal/domain"
)

// PaymentRepository implements domain.PaymentRepo using PostgreSQL.
type PaymentRepository struct {
	db *sql.DB
}

// NewPaymentRepository creates a new PaymentRepository.
func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

// Create inserts a new payment into the database.
func (r *PaymentRepository) Create(p *domain.Payment) error {
	query := `INSERT INTO payments (id, delivery_id, user_id, amount_minor, currency, status, correlation_id, causation_id, created_at, updated_at, gateway_payment_id, gateway_session_id)
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	_, err := r.db.Exec(query,
		p.ID,
		p.DeliveryID,
		p.UserID,
		p.Amount,
		p.Currency,
		string(p.Status),
		p.CorrelationID,
		p.CausationID,
		p.CreatedAt,
		p.UpdatedAt,
		p.GatewayPaymentID,
		p.GatewaySessionID,
	)
	if err != nil {
		return fmt.Errorf("failed to create payment: %w", err)
	}
	return nil
}

// Get retrieves a payment by ID.
func (r *PaymentRepository) Get(id string) (*domain.Payment, error) {
	query := `SELECT id, delivery_id, user_id, amount_minor, currency, status, correlation_id, causation_id, created_at, updated_at, gateway_payment_id, gateway_session_id FROM payments WHERE id = $1`
	row := r.db.QueryRow(query, id)

	p := &domain.Payment{}
	err := row.Scan(
		&p.ID,
		&p.DeliveryID,
		&p.UserID,
		&p.Amount,
		&p.Currency,
		&p.Status,
		&p.CorrelationID,
		&p.CausationID,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.GatewayPaymentID,
		&p.GatewaySessionID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("payment not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}
	return p, nil
}

// Update updates an existing payment.
func (r *PaymentRepository) Update(p *domain.Payment) error {
	query := `UPDATE payments SET status = $1, updated_at = $2, gateway_payment_id = $3, gateway_session_id = $4 WHERE id = $5`
	result, err := r.db.Exec(query,
		string(p.Status),
		p.UpdatedAt,
		p.GatewayPaymentID,
		p.GatewaySessionID,
		p.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("payment not found: %s", p.ID)
	}
	return nil
}

// Delete removes a payment from the database.
func (r *PaymentRepository) Delete(id string) error {
	query := `DELETE FROM payments WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete payment: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("payment not found: %s", id)
	}
	return nil
}

// List retrieves all payments.
func (r *PaymentRepository) List() ([]*domain.Payment, error) {
	query := `SELECT id, delivery_id, user_id, amount_minor, currency, status, correlation_id, causation_id, created_at, updated_at, gateway_payment_id, gateway_session_id FROM payments`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list payments: %w", err)
	}
	defer rows.Close()

	var payments []*domain.Payment
	for rows.Next() {
		p := &domain.Payment{}
		err := rows.Scan(
			&p.ID,
			&p.DeliveryID,
			&p.UserID,
			&p.Amount,
			&p.Currency,
			&p.Status,
			&p.CorrelationID,
			&p.CausationID,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.GatewayPaymentID,
			&p.GatewaySessionID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}
		payments = append(payments, p)
	}
	return payments, nil
}

// FindByDeliveryID retrieves a payment by delivery ID.
func (r *PaymentRepository) FindByDeliveryID(deliveryID string) (*domain.Payment, error) {
	query := `SELECT id, delivery_id, user_id, amount_minor, currency, status, correlation_id, causation_id, created_at, updated_at, gateway_payment_id, gateway_session_id FROM payments WHERE delivery_id = $1`
	row := r.db.QueryRow(query, deliveryID)

	p := &domain.Payment{}
	err := row.Scan(
		&p.ID,
		&p.DeliveryID,
		&p.UserID,
		&p.Amount,
		&p.Currency,
		&p.Status,
		&p.CorrelationID,
		&p.CausationID,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.GatewayPaymentID,
		&p.GatewaySessionID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("payment not found for delivery ID: %s", deliveryID)
		}
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}
	return p, nil
}

// FindByUserID retrieves all payments for a user.
func (r *PaymentRepository) FindByUserID(userID string) ([]*domain.Payment, error) {
	query := `SELECT id, delivery_id, user_id, amount_minor, currency, status, correlation_id, causation_id, created_at, updated_at, gateway_payment_id, gateway_session_id FROM payments WHERE user_id = $1`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find payments by user: %w", err)
	}
	defer rows.Close()

	var payments []*domain.Payment
	for rows.Next() {
		p := &domain.Payment{}
		err := rows.Scan(
			&p.ID,
			&p.DeliveryID,
			&p.UserID,
			&p.Amount,
			&p.Currency,
			&p.Status,
			&p.CorrelationID,
			&p.CausationID,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.GatewayPaymentID,
			&p.GatewaySessionID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}
		payments = append(payments, p)
	}
	return payments, nil
}