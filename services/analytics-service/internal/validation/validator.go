package validation

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

func ClampPage(page int) int {
	if page < 1 {
		return DefaultPage
	}
	return page
}

func ClampLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}

func ClampTopLimit(limit int) int {
	if limit <= 0 {
		return 10
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func parseTime(v any) (time.Time, error) {
	s := asString(v)
	if s == "" {
		return time.Time{}, fmt.Errorf("missing time")
	}
	return time.Parse(time.RFC3339, s)
}

// ParseRange extracts an AnalyticsRange variable into a validated TimeRange.
func ParseRange(vars map[string]any) (ports.TimeRange, error) {
	m := asMap(vars["range"])
	from, err := parseTime(m["from"])
	if err != nil {
		return ports.TimeRange{}, fmt.Errorf("range.from: %w", err)
	}
	to, err := parseTime(m["to"])
	if err != nil {
		return ports.TimeRange{}, fmt.Errorf("range.to: %w", err)
	}
	gran := ports.AnalyticsGranularity(strings.ToUpper(asString(m["granularity"])))
	r := ports.TimeRange{From: from, To: to, Granularity: gran}
	if err := r.Validate(); err != nil {
		return ports.TimeRange{}, err
	}
	return r, nil
}

func ParseDeliveryFilter(vars map[string]any) (ports.DeliveryAnalyticsFilter, error) {
	m := asMap(vars["filter"])
	rm := asMap(m["range"])
	r, err := ParseRange(map[string]any{"range": rm})
	if err != nil {
		return ports.DeliveryAnalyticsFilter{}, err
	}
	return ports.DeliveryAnalyticsFilter{
		Range:    r,
		CityID:   asString(m["cityId"]),
		DriverID: asString(m["driverId"]),
	}, nil
}

func ParseDriverFilter(vars map[string]any) (ports.DriverAnalyticsFilter, error) {
	m := asMap(vars["filter"])
	rm := asMap(m["range"])
	r, err := ParseRange(map[string]any{"range": rm})
	if err != nil {
		return ports.DriverAnalyticsFilter{}, err
	}
	return ports.DriverAnalyticsFilter{
		Range:    r,
		DriverID: asString(m["driverId"]),
	}, nil
}

func ParsePaymentFilter(vars map[string]any) (ports.PaymentAnalyticsFilter, error) {
	m := asMap(vars["filter"])
	rm := asMap(m["range"])
	r, err := ParseRange(map[string]any{"range": rm})
	if err != nil {
		return ports.PaymentAnalyticsFilter{}, err
	}
	return ports.PaymentAnalyticsFilter{
		Range:    r,
		Provider: asString(m["provider"]),
	}, nil
}

func ParsePage(vars map[string]any) (page, limit int) {
	page = DefaultPage
	limit = DefaultLimit
	if v, ok := vars["page"].(float64); ok {
		page = ClampPage(int(v))
	}
	if v, ok := vars["limit"].(float64); ok {
		limit = ClampLimit(int(v))
	}
	return page, limit
}

func ValidateSeverity(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return "", nil
	}
	switch s {
	case "ERROR", "WARNING", "INFO":
		return s, nil
	default:
		return "", fmt.Errorf("invalid severity: %s", s)
	}
}

func ScopeFromRequest(r *http.Request) string {
	if role := strings.TrimSpace(r.Header.Get("x-user-role")); role != "" {
		return "role=" + strings.ToUpper(role)
	}
	return "public"
}
