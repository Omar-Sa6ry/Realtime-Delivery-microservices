package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

type AuditRepository struct {
	db *sql.DB
}

func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) LogEvent(eventType string, paymentID string, details map[string]interface{}) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		detailsJSON = []byte("{}")
	}
	query := `INSERT INTO payment_audit (event_type, payment_id, details, created_at)
			  VALUES ($1, $2, $3, NOW())`
	_, err = r.db.Exec(query, eventType, paymentID, detailsJSON)
	if err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}
	return nil
}

func (r *AuditRepository) GetEventsByPaymentID(paymentID string) ([]map[string]interface{}, error) {
	query := `SELECT event_type, details, created_at FROM payment_audit WHERE payment_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.Query(query, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve audit events: %w", err)
	}
	defer rows.Close()

	var events []map[string]interface{}
	for rows.Next() {
		var eventType string
		var details string
		var createdAt string
		err := rows.Scan(&eventType, &details, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit event: %w", err)
		}
		event := map[string]interface{}{
			"event_type": eventType,
			"details":    details,
			"created_at": createdAt,
		}
		events = append(events, event)
	}
	return events, nil
}
