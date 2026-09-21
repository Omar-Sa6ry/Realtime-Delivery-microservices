package domain

import (
	"fmt"
	"strings"
	"time"
)

type DataQualitySeverity string

const (
	SeverityError   DataQualitySeverity = "ERROR"
	SeverityWarning DataQualitySeverity = "WARNING"
	SeverityInfo    DataQualitySeverity = "INFO"
)

func (s DataQualitySeverity) IsValid() bool {
	switch s {
	case SeverityError, SeverityWarning, SeverityInfo:
		return true
	default:
		return false
	}
}

// Data quality issue types
const (
	IssueNegativeDuration     = "negative_duration"
	IssueOutOfOrderTimestamps = "out_of_order_timestamps"
	IssueNegativeAmount       = "negative_amount"
	IssueRefundExceedsCapture = "refund_exceeds_capture"
	IssueUnknownReference     = "unknown_reference"
	IssueFutureTimestamp      = "future_timestamp"
	IssueDuplicateEvent       = "duplicate_event"
	IssueUnsupportedVersion   = "unsupported_version"
	IssueMissingAggregateID   = "missing_aggregate_id"
	IssueInvalidEnvelope      = "invalid_envelope"
	IssueUnknownEventType     = "unknown_event_type"
)

type DataQualityIssue struct {
	IssueID       string
	EventID       string
	IssueType     string
	AggregateType string
	AggregateID   string
	DetectedAt    time.Time
	Severity      DataQualitySeverity
	Details       string
	ResolvedAt    *time.Time
}

func NewDataQualityIssue(issueID, eventID, issueType, aggregateType, aggregateID string, severity DataQualitySeverity, details string) *DataQualityIssue {
	return &DataQualityIssue{
		IssueID:       issueID,
		EventID:       eventID,
		IssueType:     issueType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		DetectedAt:    time.Now().UTC(),
		Severity:      severity,
		Details:       details,
	}
}

func (d *DataQualityIssue) Validate() error {
	if d == nil {
		return ErrInvalidEnvelope
	}
	if strings.TrimSpace(d.IssueID) == "" {
		return ErrMissingIssueID
	}
	if strings.TrimSpace(d.IssueType) == "" {
		return ErrMissingIssueType
	}
	if !d.Severity.IsValid() {
		return fmt.Errorf("%w: %s", ErrInvalidIssueSeverity, d.Severity)
	}
	if d.DetectedAt.IsZero() {
		return ErrInvalidOccurredAt
	}
	return nil
}

func (d *DataQualityIssue) IsResolved() bool {
	return d != nil && d.ResolvedAt != nil
}

func (d *DataQualityIssue) MarkResolved(at time.Time) {
	if d == nil {
		return
	}
	d.ResolvedAt = &at
}
