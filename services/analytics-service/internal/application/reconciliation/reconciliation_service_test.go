package reconciliation

import (
	"context"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	queries := testutil.NewMockReconciliationQueries()

	svc := NewService(queries, Config{
		WindowHours:          2,
		FreshWarnSeconds:     120,
		FreshDegradedSeconds: 600,
		MinCoverageRatio:     0.95,
	})

	assert.NotNil(t, svc)
	assert.Equal(t, 2, svc.cfg.WindowHours)
	assert.Equal(t, int64(120), svc.cfg.FreshWarnSeconds)
	assert.Equal(t, int64(600), svc.cfg.FreshDegradedSeconds)
	assert.Equal(t, 0.95, svc.cfg.MinCoverageRatio)
}

func TestNewService_Defaults(t *testing.T) {
	queries := testutil.NewMockReconciliationQueries()

	svc := NewService(queries, Config{})

	assert.Equal(t, 1, svc.cfg.WindowHours)
	assert.Equal(t, int64(DefaultFreshWarnSeconds), svc.cfg.FreshWarnSeconds)
	assert.Equal(t, int64(DefaultFreshDegradedSeconds), svc.cfg.FreshDegradedSeconds)
	assert.Equal(t, 0.99, svc.cfg.MinCoverageRatio)
}

func TestService_Check_Healthy(t *testing.T) {
	queries := testutil.NewMockReconciliationQueries()
	now := time.Now().UTC()
	queries.SetMaxRawOccurredAt(now.Add(-30 * time.Second))
	queries.SetCountRawEvents(1000)
	queries.SetCountFactDeliveryEvents(995)
	queries.SetCountDataQualityByType(5)
	queries.SetDistinctRawEventTypes([]string{"delivery.created", "delivery.completed"})
	queries.SetOpenIssuesBySeverity(map[string]int64{"ERROR": 2, "WARNING": 5})

	svc := NewService(queries, Config{WindowHours: 1})

	report, err := svc.Check(context.Background())

	require.NoError(t, err)
	assert.Equal(t, FreshnessHealthy, report.Freshness)
	assert.True(t, report.FreshnessSeconds < 60)
	assert.Equal(t, int64(1000), report.RawTotal)
	assert.Equal(t, int64(995), report.FactTotal)
	assert.InDelta(t, 0.995, report.CoverageRatio, 0.001)
	assert.False(t, report.GapDetected)
	assert.Equal(t, int64(5), report.DuplicateCount)
	assert.Len(t, report.UnknownTypes, 0)
	assert.Equal(t, int64(2), report.OpenIssues["ERROR"])
	assert.Equal(t, int64(5), report.OpenIssues["WARNING"])
}

func TestService_Check_WarningFreshness(t *testing.T) {
	queries := testutil.NewMockReconciliationQueries()
	now := time.Now().UTC()
	queries.SetMaxRawOccurredAt(now.Add(-90 * time.Second)) // Between warn (60) and degraded (300)
	queries.SetCountRawEvents(1000)
	queries.SetCountFactDeliveryEvents(995)
	queries.SetCountDataQualityByType(5)
	queries.SetDistinctRawEventTypes([]string{"delivery.created"})
	queries.SetOpenIssuesBySeverity(map[string]int64{})

	svc := NewService(queries, Config{
		WindowHours:          1,
		FreshWarnSeconds:     60,
		FreshDegradedSeconds: 300,
	})

	report, err := svc.Check(context.Background())

	require.NoError(t, err)
	assert.Equal(t, FreshnessWarning, report.Freshness)
	assert.True(t, report.FreshnessSeconds >= 60 && report.FreshnessSeconds < 300)
}

func TestService_Check_DegradedFreshness(t *testing.T) {
	queries := testutil.NewMockReconciliationQueries()
	now := time.Now().UTC()
	queries.SetMaxRawOccurredAt(now.Add(-400 * time.Second)) // Beyond degraded (300)
	queries.SetCountRawEvents(1000)
	queries.SetCountFactDeliveryEvents(995)
	queries.SetCountDataQualityByType(5)
	queries.SetDistinctRawEventTypes([]string{"delivery.created"})
	queries.SetOpenIssuesBySeverity(map[string]int64{})

	svc := NewService(queries, Config{
		WindowHours:          1,
		FreshWarnSeconds:     60,
		FreshDegradedSeconds: 300,
	})

	report, err := svc.Check(context.Background())

	require.NoError(t, err)
	assert.Equal(t, FreshnessDegraded, report.Freshness)
	assert.True(t, report.FreshnessSeconds >= 300)
}

func TestService_Check_GapDetected(t *testing.T) {
	queries := testutil.NewMockReconciliationQueries()
	now := time.Now().UTC()
	queries.SetMaxRawOccurredAt(now.Add(-30 * time.Second))
	queries.SetCountRawEvents(1000)
	queries.SetCountFactDeliveryEvents(950) // 95% coverage, below 99% threshold
	queries.SetCountDataQualityByType(5)
	queries.SetDistinctRawEventTypes([]string{"delivery.created"})
	queries.SetOpenIssuesBySeverity(map[string]int64{})

	svc := NewService(queries, Config{
		WindowHours:      1,
		MinCoverageRatio: 0.99,
	})

	report, err := svc.Check(context.Background())

	require.NoError(t, err)
	assert.True(t, report.GapDetected)
	assert.InDelta(t, 0.95, report.CoverageRatio, 0.001)
}

func TestService_Check_UnknownEventTypes(t *testing.T) {
	queries := testutil.NewMockReconciliationQueries()
	now := time.Now().UTC()
	queries.SetMaxRawOccurredAt(now.Add(-30 * time.Second))
	queries.SetCountRawEvents(1000)
	queries.SetCountFactDeliveryEvents(995)
	queries.SetCountDataQualityByType(5)
	queries.SetDistinctRawEventTypes([]string{"delivery.created", "unknown.event.type", "another.unknown"})
	queries.SetOpenIssuesBySeverity(map[string]int64{})

	svc := NewService(queries, Config{WindowHours: 1})

	report, err := svc.Check(context.Background())

	require.NoError(t, err)
	assert.Len(t, report.UnknownTypes, 2)
	assert.Contains(t, report.UnknownTypes, "unknown.event.type")
	assert.Contains(t, report.UnknownTypes, "another.unknown")
}

func TestService_Check_ZeroRawEvents(t *testing.T) {
	queries := testutil.NewMockReconciliationQueries()
	now := time.Now().UTC()
	queries.SetMaxRawOccurredAt(now.Add(-30 * time.Second))
	queries.SetCountRawEvents(0)
	queries.SetCountFactDeliveryEvents(0)
	queries.SetCountDataQualityByType(0)
	queries.SetDistinctRawEventTypes([]string{})
	queries.SetOpenIssuesBySeverity(map[string]int64{})

	svc := NewService(queries, Config{WindowHours: 1})

	report, err := svc.Check(context.Background())

	require.NoError(t, err)
	assert.Equal(t, int64(0), report.RawTotal)
	assert.Equal(t, int64(0), report.FactTotal)
	assert.Equal(t, float64(0), report.CoverageRatio)
	assert.False(t, report.GapDetected)
}

func TestService_Check_QueryErrors(t *testing.T) {
	queries := testutil.NewMockReconciliationQueries()
	queries.SetError(assert.AnError)

	svc := NewService(queries, Config{WindowHours: 1})

	_, err := svc.Check(context.Background())

	assert.Error(t, err)
}

func TestGradeFreshness(t *testing.T) {
	tests := []struct {
		name       string
		seconds    float64
		warn       int64
		degraded   int64
		expected   FreshnessStatus
	}{
		{"healthy", 30, 60, 300, FreshnessHealthy},
		{"warning boundary", 60, 60, 300, FreshnessWarning},
		{"warning", 120, 60, 300, FreshnessWarning},
		{"degraded boundary", 300, 60, 300, FreshnessDegraded},
		{"degraded", 400, 60, 300, FreshnessDegraded},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := gradeFreshness(tc.seconds, tc.warn, tc.degraded)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsKnownEventType(t *testing.T) {
	tests := []struct {
		name     string
		eventType string
		expected bool
	}{
		{"delivery.created", "delivery.created", true},
		{"delivery.completed", "delivery.completed", true},
		{"driver.available", "driver.available", true},
		{"driver.assignment.offered", "driver.assignment.offered", true},
		{"payment.created", "payment.created", true},
		{"payment.captured", "payment.captured", true},
		{"notification.sent", "notification.sent", true},
		{"unknown.event", "unknown.event", false},
		{"", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isKnownEventType(tc.eventType))
		})
	}
}