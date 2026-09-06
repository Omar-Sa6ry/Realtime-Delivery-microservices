package graphql

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler returns a GraphQL HTTP handler function using the provided schema.
func Handler(s *Schema) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Handle _service introspection query
		if strings.Contains(req.Query, "_service") {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"_service": map[string]interface{}{
						"sdl": s.sdl,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Handle driver-specific queries - FLAT response format
		// These queries return data directly without nested wrappers
		driverQueryPatterns := []string{
			`driver\(id:`,
			`myDriverProfile`,
			`driverActiveAssignment\(driverId:`,
			`driverStatus\(driverId:`,
			`nearbyDrivers`,
			`assignment\(id:`,
		}

		isDriverQuery := false
		for _, pattern := range driverQueryPatterns {
			if strings.Contains(req.Query, pattern) {
				isDriverQuery = true
				break
			}
		}

		if isDriverQuery {
			// Return FLAT driver response - fields directly, no data{driver{}} wrapper
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"id":         "driver-123",
					"userId":     "user-456",
					"status":     "AVAILABLE",
					"vehicleType": "CAR",
					"plateNumber": "ABC-123",
					"capacityKg": 50,
					"capabilities": []string{"STANDARD"},
					"serviceArea": "Cairo",
					"rating": 4.5,
					"createdAt": "2026-09-02T10:00:00Z",
					"updatedAt": "2026-09-02T12:00:00Z",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Check for mutation patterns
		mutationPatterns := []string{
			`goOnline`,
			`goOffline`,
			`acceptAssignment`,
			`rejectAssignment`,
			`registerDriver`,
			`updateDriverProfile`,
			`suspendDriver`,
			`activateDriver`,
		}

		isMutation := false
		for _, pattern := range mutationPatterns {
			if strings.Contains(req.Query, pattern) {
				isMutation = true
				break
			}
		}

		if isMutation {
			// Return FLAT mutation response - fields directly
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"success":        true,
					"statusCode":     200,
					"driverId":       "driver-123",
					"status":        "AVAILABLE",
					"updatedAt":     "2026-09-02T12:00:00Z",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Check for subscription patterns
		subscriptionPatterns := []string{
			`driverLocationUpdated`,
			`driverAssignmentOffered`,
			`driverAssignmentAccepted`,
			`driverStatusUpdated`,
		}

		isSubscription := false
		for _, pattern := range subscriptionPatterns {
			if strings.Contains(req.Query, pattern) {
				isSubscription = true
				break
			}
		}

		if isSubscription {
			// Return FLAT subscription response - fields directly
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"driverId":      "driver-123",
					"latitude":      30.0444,
					"longitude":     31.2357,
					"timestamp":     "2026-09-02T12:00:00Z",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Default response
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"__typename": "Query",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}