package i18n

// English messages (same pattern as payment-service).
var enMessages = map[string]string{
	// Server / System
	"server.healthy": "Analytics service is healthy",
	"server.running": "Analytics service is running",

	// Queries
	"analytics.found":              "Analytics retrieved successfully",
	"platform.overview.found":      "Platform overview retrieved successfully",
	"delivery.analytics.found":     "Delivery analytics retrieved successfully",
	"driver.analytics.found":       "Driver analytics retrieved successfully",
	"driver.top.found":             "Top drivers retrieved successfully",
	"payment.analytics.found":      "Payment analytics retrieved successfully",
	"raw.events.found":             "Raw events retrieved successfully",
	"data.quality.issues.found":    "Data quality issues retrieved successfully",

	// Validation errors
	"error.range.from.missing":     "Query range 'from' is required",
	"error.range.to.missing":       "Query range 'to' is required",
	"error.range.invalid":          "'from' must be before 'to'",
	"error.severity.invalid":       "Severity must be one of: ERROR, WARNING, INFO",
	"error.limit.invalid":          "Limit must be between 1 and 100",

	// Generic errors
	"error.internal":               "An internal server error occurred",
	"error.unauthorized":           "Unauthorized: admin role required",
	"error.not.found":              "The requested resource was not found",
}
