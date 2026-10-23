package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataQualitySeverity_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		severity DataQualitySeverity
		expected bool
	}{
		{"error", SeverityError, true},
		{"warning", SeverityWarning, true},
		{"info", SeverityInfo, true},
		{"invalid", DataQualitySeverity("INVALID"), false},
		{"empty", DataQualitySeverity(""), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.severity.IsValid())
		})
	}
}

func TestNewDataQualityIssue(t *testing.T) {
	now := time.Now().UTC()
	issue := NewDataQualityIssue(
		"issue-123",
		"evt-123",
		IssueNegativeDuration,
		"delivery",
		"delivery-456",
		SeverityWarning,
		"test details",
	)

	require.NotNil(t, issue)
	assert.Equal(t, "issue-123", issue.IssueID)
	assert.Equal(t, "evt-123", issue.EventID)
	assert.Equal(t, IssueNegativeDuration, issue.IssueType)
	assert.Equal(t, "delivery", issue.AggregateType)
	assert.Equal(t, "delivery-456", issue.AggregateID)
	assert.Equal(t, SeverityWarning, issue.Severity)
	assert.Equal(t, "test details", issue.Details)
	assert.True(t, issue.DetectedAt.After(now.Add(-time.Second)))
	assert.Nil(t, issue.ResolvedAt)
}

func TestDataQualityIssue_Validate(t *testing.T) {
	base := DataQualityIssue{
		IssueID:       "issue-123",
		EventID:       "evt-123",
		IssueType:     IssueNegativeDuration,
		AggregateType: "delivery",
		AggregateID:   "delivery-456",
		DetectedAt:    time.Now().UTC(),
		Severity:      SeverityWarning,
		Details:       "test details",
	}

	tests := []struct {
		name        string
		issue       *DataQualityIssue
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid issue",
			issue:       &base,
			expectError: false,
		},
		{
			name: "nil issue",
			issue:       nil,
			expectError: true,
			errorMsg:    "invalid event envelope",
		},
		{
			name: "empty issue ID",
			issue:       func() *DataQualityIssue { i := base; i.IssueID = ""; return &i }(),
			expectError: true,
			errorMsg:    "issueId is required",
		},
		{
			name: "empty issue type",
			issue:       func() *DataQualityIssue { i := base; i.IssueType = ""; return &i }(),
			expectError: true,
			errorMsg:    "issueType is required",
		},
		{
			name: "invalid severity",
			issue:       func() *DataQualityIssue { i := base; i.Severity = "INVALID"; return &i }(),
			expectError: true,
			errorMsg:    "invalid data quality severity",
		},
		{
			name: "zero detected at",
			issue:       func() *DataQualityIssue { i := base; i.DetectedAt = time.Time{}; return &i }(),
			expectError: true,
			errorMsg:    "occurredAt is invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.issue.Validate()
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDataQualityIssue_IsResolved(t *testing.T) {
	issue := &DataQualityIssue{
		IssueID:    "issue-123",
		DetectedAt: time.Now().UTC(),
	}

	assert.False(t, issue.IsResolved())

	resolvedAt := time.Now().UTC()
	issue.MarkResolved(resolvedAt)
	assert.True(t, issue.IsResolved())
	assert.Equal(t, resolvedAt, *issue.ResolvedAt)
}

func TestDataQualityIssue_MarkResolved_Nil(t *testing.T) {
	var issue *DataQualityIssue
	issue.MarkResolved(time.Now().UTC())
	// Should not panic
}