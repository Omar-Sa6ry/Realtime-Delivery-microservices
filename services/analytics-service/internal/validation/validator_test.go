package validation

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClampPage(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"positive", 5, 5},
		{"one", 1, 1},
		{"zero", 0, DefaultPage},
		{"negative", -5, DefaultPage},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ClampPage(tc.input))
		})
	}
}

func TestClampLimit(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"within range", 50, 50},
		{"default", 0, DefaultLimit},
		{"negative", -10, DefaultLimit},
		{"above max", 150, MaxLimit},
		{"at max", 100, MaxLimit},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ClampLimit(tc.input))
		})
	}
}

func TestClampTopLimit(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"within range", 50, 50},
		{"default", 0, 10},
		{"negative", -10, 10},
		{"above max", 150, MaxLimit},
		{"at max", 100, MaxLimit},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ClampTopLimit(tc.input))
		})
	}
}

func TestParseRange(t *testing.T) {
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now

	tests := []struct {
		name        string
		query       string
		vars        map[string]any
		expectError bool
		checkRange  func(*testing.T, ports.TimeRange)
	}{
		{
			name: "from variables",
			query: `query { platformOverview(range: {from: $from, to: $to, granularity: $gran}) }`,
			vars: map[string]any{
				"range": map[string]any{
					"from":        from.Format(time.RFC3339),
					"to":          to.Format(time.RFC3339),
					"granularity": "DAY",
				},
			},
			expectError: false,
			checkRange: func(t *testing.T, r ports.TimeRange) {
				assert.True(t, r.From.Equal(from) || r.From.Sub(from).Abs() < time.Second)
				assert.True(t, r.To.Equal(to) || r.To.Sub(to).Abs() < time.Second)
				assert.Equal(t, ports.GranularityDay, r.Granularity)
			},
		},
		{
			name: "from query literals",
			query: `query { platformOverview(range: {from: "2024-01-14T10:00:00Z", to: "2024-01-15T10:00:00Z", granularity: HOUR}) }`,
			vars:  map[string]any{},
			expectError: false,
			checkRange: func(t *testing.T, r ports.TimeRange) {
				expectedFrom, _ := time.Parse(time.RFC3339, "2024-01-14T10:00:00Z")
				expectedTo, _ := time.Parse(time.RFC3339, "2024-01-15T10:00:00Z")
				assert.True(t, r.From.Equal(expectedFrom))
				assert.True(t, r.To.Equal(expectedTo))
				assert.Equal(t, ports.GranularityHour, r.Granularity)
			},
		},
		{
			name: "missing from",
			query: `query { platformOverview(range: {to: "2024-01-15T10:00:00Z", granularity: DAY}) }`,
			vars:  map[string]any{},
			expectError: true,
		},
		{
			name: "invalid granularity",
			query: `query { platformOverview(range: {from: "2024-01-14T10:00:00Z", to: "2024-01-15T10:00:00Z", granularity: INVALID}) }`,
			vars:  map[string]any{},
			expectError: true,
		},
		{
			name: "to before from",
			query: `query { platformOverview(range: {from: "2024-01-15T10:00:00Z", to: "2024-01-14T10:00:00Z", granularity: DAY}) }`,
			vars:  map[string]any{},
			expectError: true,
		},
		{
			name: "range exceeds max",
			query: `query { platformOverview(range: {from: "2023-01-01T10:00:00Z", to: "2024-01-15T10:00:00Z", granularity: DAY}) }`,
			vars:  map[string]any{},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseRange(tc.query, tc.vars)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.checkRange != nil {
					tc.checkRange(t, result)
				}
			}
		})
	}
}

func TestParseDeliveryFilter(t *testing.T) {
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now

	query := `query { deliveryAnalytics(filter: {range: {from: $from, to: $to, granularity: $gran}, cityId: $cityId, driverId: $driverId}) }`
	vars := map[string]any{
		"filter": map[string]any{
			"range": map[string]any{
				"from":        from.Format(time.RFC3339),
				"to":          to.Format(time.RFC3339),
				"granularity": "DAY",
			},
			"cityId":   "city-1",
			"driverId": "driver-1",
		},
	}

	result, err := ParseDeliveryFilter(query, vars)

	require.NoError(t, err)
	assert.Equal(t, "city-1", result.CityID)
	assert.Equal(t, "driver-1", result.DriverID)
	assert.Equal(t, ports.GranularityDay, result.Range.Granularity)
}

func TestParseDriverFilter(t *testing.T) {
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now

	query := `query { driverAnalytics(filter: {range: {from: $from, to: $to, granularity: $gran}, driverId: $driverId}) }`
	vars := map[string]any{
		"filter": map[string]any{
			"range": map[string]any{
				"from":        from.Format(time.RFC3339),
				"to":          to.Format(time.RFC3339),
				"granularity": "DAY",
			},
			"driverId": "driver-1",
		},
		"driverId": "driver-1",
	}

	result, err := ParseDriverFilter(query, vars)

	require.NoError(t, err)
	assert.Equal(t, "driver-1", result.DriverID)
	assert.Equal(t, ports.GranularityDay, result.Range.Granularity)
}

func TestParsePaymentFilter(t *testing.T) {
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now

	query := `query { paymentAnalytics(filter: {range: {from: $from, to: $to, granularity: $gran}, provider: $provider}) }`
	vars := map[string]any{
		"filter": map[string]any{
			"range": map[string]any{
				"from":        from.Format(time.RFC3339),
				"to":          to.Format(time.RFC3339),
				"granularity": "DAY",
			},
			"provider": "STRIPE",
		},
	}

	result, err := ParsePaymentFilter(query, vars)

	require.NoError(t, err)
	assert.Equal(t, "STRIPE", result.Provider)
	assert.Equal(t, ports.GranularityDay, result.Range.Granularity)
}

func TestParsePaymentFilter_InvalidProvider(t *testing.T) {
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now

	query := `query { paymentAnalytics(filter: {range: {from: $from, to: $to, granularity: $gran}, provider: $provider}) }`
	vars := map[string]any{
		"filter": map[string]any{
			"range": map[string]any{
				"from":        from.Format(time.RFC3339),
				"to":          to.Format(time.RFC3339),
				"granularity": "DAY",
			},
			"provider": "INVALID",
		},
	}

	_, err := ParsePaymentFilter(query, vars)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid provider")
}

func TestParseLimit(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		vars     map[string]any
		def      int
		expected int
	}{
		{"from vars", `query { topDrivers(limit: $limit) }`, map[string]any{"limit": float64(20)}, 10, 20},
		{"from query", `query { topDrivers(limit: 50) }`, map[string]any{}, 10, 50},
		{"clamped to max", `query { topDrivers(limit: 200) }`, map[string]any{}, 10, MaxLimit},
		{"default", `query { topDrivers }`, map[string]any{}, 10, 10},
		{"zero from vars", `query { topDrivers(limit: $limit) }`, map[string]any{"limit": float64(0)}, 10, 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseLimit(tc.query, tc.vars, tc.def)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestParsePage(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		vars       map[string]any
		expPage    int
		expLimit   int
	}{
		{"from vars", `query { rawAnalyticsEvents(page: $page, limit: $limit) }`, map[string]any{"page": float64(2), "limit": float64(50)}, 2, 50},
		{"from query", `query { rawAnalyticsEvents(page: 3, limit: 25) }`, map[string]any{}, 3, 25},
		{"defaults", `query { rawAnalyticsEvents }`, map[string]any{}, DefaultPage, DefaultLimit},
		{"clamped page", `query { rawAnalyticsEvents(page: -1) }`, map[string]any{}, DefaultPage, DefaultLimit},
		{"clamped limit", `query { rawAnalyticsEvents(limit: 200) }`, map[string]any{}, DefaultPage, MaxLimit},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, limit := ParsePage(tc.query, tc.vars)
			assert.Equal(t, tc.expPage, page)
			assert.Equal(t, tc.expLimit, limit)
		})
	}
}

func TestExtractQueryField(t *testing.T) {
	query := `query { rawAnalyticsEvents(eventType: "DELIVERY_CREATED", severity: "ERROR", from: "2024-01-14T10:00:00Z", to: "2024-01-15T10:00:00Z") }`
	vars := map[string]any{}

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"eventType", "eventType", "DELIVERY_CREATED"},
		{"severity", "severity", "ERROR"},
		{"from", "from", "2024-01-14T10:00:00Z"},
		{"to", "to", "2024-01-15T10:00:00Z"},
		{"unknown", "unknown", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractQueryField(query, tc.key, vars)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidateSeverity(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expected    string
	}{
		{"ERROR", "ERROR", false, "ERROR"},
		{"WARNING", "WARNING", false, "WARNING"},
		{"INFO", "INFO", false, "INFO"},
		{"lowercase error", "error", false, "ERROR"},
		{"whitespace", "  WARNING  ", false, "WARNING"},
		{"empty", "", false, ""},
		{"invalid", "INVALID", true, ""},
		{"DEBUG", "DEBUG", true, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ValidateSeverity(tc.input)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestNormalizeEventType(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expected    string
	}{
		{"enum uppercase", "DELIVERY_CREATED", false, "delivery.created"},
		{"enum uppercase driver", "DRIVER_ASSIGNMENT_OFFERED", false, "driver.assignment.offered"},
		{"enum uppercase payment", "PAYMENT_CAPTURED", false, "payment.captured"},
		{"enum uppercase notification", "NOTIFICATION_SENT", false, "notification.sent"},
		{"dot notation", "delivery.created", false, "delivery.created"},
		{"whitespace", "  DELIVERY_CREATED  ", false, "delivery.created"},
		{"empty", "", false, ""},
		{"invalid", "INVALID_EVENT", true, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := NormalizeEventType(tc.input)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestNormalizeProvider(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expected    string
	}{
		{"STRIPE", "STRIPE", false, "STRIPE"},
		{"PAYPAL", "PAYPAL", false, "PAYPAL"},
		{"CASH", "CASH", false, "CASH"},
		{"lowercase", "stripe", false, "STRIPE"},
		{"whitespace", "  paypal  ", false, "PAYPAL"},
		{"empty", "", false, ""},
		{"invalid", "INVALID", true, ""},
		{"apple pay", "APPLE_PAY", true, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := NormalizeProvider(tc.input)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestScopeFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected string
	}{
		{"X-Scope header", map[string]string{"X-Scope": "tenant-123"}, "tenant-123"},
		{"no header", map[string]string{}, "global"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			result := ScopeFromRequest(req)
			assert.Equal(t, tc.expected, result)
		})
	}
}