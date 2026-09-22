package validation

import (
	"fmt"
	"net/http"
	"regexp"
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

var (
	reFrom        = regexp.MustCompile(`(?i)from\s*:\s*["']([^"']+)["']`)
	reTo          = regexp.MustCompile(`(?i)to\s*:\s*["']([^"']+)["']`)
	reGranularity = regexp.MustCompile(`(?i)granularity\s*:\s*([A-Za-z0-9_]+)`)
	reDriverID    = regexp.MustCompile(`(?i)driverId\s*:\s*(?:["']([^"']+)["']|([$A-Za-z0-9_-]+))`)
	reCityID      = regexp.MustCompile(`(?i)cityId\s*:\s*["']([^"']+)["']`)
	reProvider    = regexp.MustCompile(`(?i)provider\s*:\s*["']?([A-Za-z0-9_]+)["']?`)
	rePage        = regexp.MustCompile(`(?i)page\s*:\s*(\d+)`)
	reLimit       = regexp.MustCompile(`(?i)limit\s*:\s*(\d+)`)
	reSeverity    = regexp.MustCompile(`(?i)severity\s*:\s*["']?([A-Za-z0-9_]+)["']?`)
	reEventType   = regexp.MustCompile(`(?i)eventType\s*:\s*["']?([A-Za-z0-9_.]+)["']?`)
)

func extractLiteral(query string, re *regexp.Regexp) string {
	m := re.FindStringSubmatch(query)
	if len(m) > 1 && m[1] != "" {
		return strings.TrimSpace(m[1])
	}
	if len(m) > 2 && m[2] != "" {
		return strings.TrimSpace(m[2])
	}
	return ""
}

func ParseRange(query string, vars map[string]any) (ports.TimeRange, error) {
	m := asMap(vars["range"])
	fromStr := asString(m["from"])
	if fromStr == "" {
		fromStr = extractLiteral(query, reFrom)
	}
	from, err := parseTime(fromStr)
	if err != nil {
		return ports.TimeRange{}, fmt.Errorf("range.from: %w", err)
	}

	toStr := asString(m["to"])
	if toStr == "" {
		toStr = extractLiteral(query, reTo)
	}
	to, err := parseTime(toStr)
	if err != nil {
		return ports.TimeRange{}, fmt.Errorf("range.to: %w", err)
	}

	granStr := asString(m["granularity"])
	if granStr == "" {
		granStr = extractLiteral(query, reGranularity)
	}
	gran := ports.AnalyticsGranularity(strings.ToUpper(granStr))
	r := ports.TimeRange{From: from, To: to, Granularity: gran}
	if err := r.Validate(); err != nil {
		return ports.TimeRange{}, err
	}
	return r, nil
}

func ParseDeliveryFilter(query string, vars map[string]any) (ports.DeliveryAnalyticsFilter, error) {
	m := asMap(vars["filter"])
	rm := asMap(m["range"])
	r, err := ParseRange(query, map[string]any{"range": rm})
	if err != nil {
		return ports.DeliveryAnalyticsFilter{}, err
	}

	cityID := asString(m["cityId"])
	if cityID == "" {
		cityID = extractLiteral(query, reCityID)
	}
	driverID := asString(m["driverId"])
	if driverID == "" {
		driverID = extractLiteral(query, reDriverID)
	}

	return ports.DeliveryAnalyticsFilter{
		Range:    r,
		CityID:   cityID,
		DriverID: driverID,
	}, nil
}

func ParseDriverFilter(query string, vars map[string]any) (ports.DriverAnalyticsFilter, error) {
	m := asMap(vars["filter"])
	rm := asMap(m["range"])
	r, err := ParseRange(query, map[string]any{"range": rm})
	if err != nil {
		return ports.DriverAnalyticsFilter{}, err
	}

	driverID := asString(vars["driverId"])
	if driverID == "" {
		driverID = asString(m["driverId"])
	}
	if driverID == "" {
		driverID = extractLiteral(query, reDriverID)
	}

	return ports.DriverAnalyticsFilter{
		Range:    r,
		DriverID: driverID,
	}, nil
}

func ParsePaymentFilter(query string, vars map[string]any) (ports.PaymentAnalyticsFilter, error) {
	m := asMap(vars["filter"])
	rm := asMap(m["range"])
	r, err := ParseRange(query, map[string]any{"range": rm})
	if err != nil {
		return ports.PaymentAnalyticsFilter{}, err
	}

	provider := asString(m["provider"])
	if provider == "" {
		provider = extractLiteral(query, reProvider)
	}
	normProvider, err := NormalizeProvider(provider)
	if err != nil {
		return ports.PaymentAnalyticsFilter{}, err
	}

	return ports.PaymentAnalyticsFilter{
		Range:    r,
		Provider: normProvider,
	}, nil
}

func ParseLimit(query string, vars map[string]any, def int) int {
	if v, ok := vars["limit"].(float64); ok {
		return ClampTopLimit(int(v))
	}
	if s := extractLiteral(query, reLimit); s != "" {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
			return ClampTopLimit(n)
		}
	}
	return def
}

func ParsePage(query string, vars map[string]any) (page, limit int) {
	page = DefaultPage
	limit = DefaultLimit
	if v, ok := vars["page"].(float64); ok {
		page = ClampPage(int(v))
	} else if s := extractLiteral(query, rePage); s != "" {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
			page = ClampPage(n)
		}
	}

	if v, ok := vars["limit"].(float64); ok {
		limit = ClampLimit(int(v))
	} else if s := extractLiteral(query, reLimit); s != "" {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
			limit = ClampLimit(n)
		}
	}
	return page, limit
}

func ExtractQueryField(query, key string, vars map[string]any) string {
	if v, ok := vars[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	switch key {
	case "eventType":
		return extractLiteral(query, reEventType)
	case "severity":
		return extractLiteral(query, reSeverity)
	case "from":
		return extractLiteral(query, reFrom)
	case "to":
		return extractLiteral(query, reTo)
	default:
		return ""
	}
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
		return "", fmt.Errorf("invalid severity %q: must be ERROR, WARNING, or INFO", s)
	}
}

// Map of GraphQL enum string to internal dot-notated topic / event_type string
var eventTypeEnumToInternal = map[string]string{
	"DELIVERY_CREATED":                 "delivery.created",
	"DELIVERY_DRIVER_ASSIGNED":         "delivery.driver.assigned",
	"DELIVERY_DRIVER_ACCEPTED":         "delivery.driver.accepted",
	"DELIVERY_PICKUP_STARTED":          "delivery.pickup.started",
	"DELIVERY_PICKED_UP":               "delivery.picked_up",
	"DELIVERY_IN_TRANSIT":              "delivery.in_transit",
	"DELIVERY_COMPLETED":               "delivery.completed",
	"DELIVERY_CANCELLED":               "delivery.cancelled",
	"DELIVERY_FAILED":                  "delivery.failed",
	"DELIVERY_DELETED":                 "delivery.deleted",
	"DRIVER_AVAILABLE":                 "driver.available",
	"DRIVER_UNAVAILABLE":               "driver.unavailable",
	"DRIVER_ASSIGNMENT_OFFERED":        "driver.assignment.offered",
	"DRIVER_ASSIGNMENT_ACCEPTED":       "driver.assignment.accepted",
	"DRIVER_ASSIGNMENT_REJECTED":       "driver.assignment.rejected",
	"DRIVER_ASSIGNMENT_EXPIRED":        "driver.assignment.expired",
	"DRIVER_ASSIGNMENT_RELEASED":       "driver.assignment.released",
	"PAYMENT_CREATED":                  "payment.created",
	"PAYMENT_AUTHORIZATION_STARTED":    "payment.authorization.started",
	"PAYMENT_AUTHORIZED":               "payment.authorized",
	"PAYMENT_AUTHORIZATION_FAILED":     "payment.authorization.failed",
	"PAYMENT_CAPTURE_STARTED":          "payment.capture.started",
	"PAYMENT_CAPTURED":                 "payment.captured",
	"PAYMENT_CAPTURE_FAILED":           "payment.capture.failed",
	"PAYMENT_CANCELLED":                "payment.cancelled",
	"PAYMENT_REFUND_STARTED":           "payment.refund.started",
	"PAYMENT_REFUNDED":                 "payment.refunded",
	"PAYMENT_REFUND_FAILED":            "payment.refund.failed",
	"PAYMENT_FAILED":                   "payment.failed",
	"NOTIFICATION_CREATED":             "notification.created",
	"NOTIFICATION_SENT":                "notification.sent",
	"NOTIFICATION_DELIVERED":           "notification.delivered",
	"NOTIFICATION_FAILED":              "notification.failed",
	"NOTIFICATION_RETRYING":            "notification.retrying",
}

func NormalizeEventType(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	// If passed as uppercase enum (e.g. PAYMENT_CAPTURED)
	upper := strings.ToUpper(s)
	if internal, ok := eventTypeEnumToInternal[upper]; ok {
		return internal, nil
	}
	// If already in dot notation (e.g. payment.captured), check if reverse exists
	lower := strings.ToLower(s)
	for _, val := range eventTypeEnumToInternal {
		if val == lower {
			return val, nil
		}
	}
	return "", fmt.Errorf("invalid eventType %q", s)
}

func NormalizeProvider(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	upper := strings.ToUpper(s)
	switch upper {
	case "STRIPE", "PAYPAL", "CASH":
		// Return lowercase or uppercase matching existing query repo expectations (lowercased in provider filter)
		return strings.ToLower(upper), nil
	default:
		return "", fmt.Errorf("invalid provider %q: must be STRIPE, PAYPAL, or CASH", s)
	}
}

func ScopeFromRequest(r *http.Request) string {
	if s := r.Header.Get("X-Scope"); s != "" {
		return s
	}
	return "global"
}
