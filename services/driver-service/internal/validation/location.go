package validation

import (
	"fmt"
	"regexp"
)

// ValidateLocation validates driver location coordinates.
func ValidateLocation(latitude, longitude string) error {
	latRegex := regexp.MustCompile(`^[-]?[0-9]\d{0,2}\.?\d*$`)
	lngRegex := regexp.MustCompile(`^[-]?(1[0-7]\d{1}\.\d*|[0-9]?\d{1,2}\.\d*)$`)

	var lat float64
	var lng float64

	if _, err := fmt.Sscanf(latitude, "%f", &lat); err != nil {
		return fmt.Errorf("invalid latitude format: %s", latitude)
	}
	if _, err := fmt.Sscanf(longitude, "%f", &lng); err != nil {
		return fmt.Errorf("invalid longitude format: %s", longitude)
	}

	if lat < -90 || lat > 90 {
		return fmt.Errorf("latitude must be between -90 and 90, got: %f", lat)
	}
	if lng < -180 || lng > 180 {
		return fmt.Errorf("longitude must be between -180 and 180, got: %f", lng)
	}

	if !latRegex.MatchString(latitude) {
		return fmt.Errorf("latitude format invalid: %s", latitude)
	}
	if !lngRegex.MatchString(longitude) {
		return fmt.Errorf("longitude format invalid: %s", longitude)
	}

	return nil
}

// ValidateDriverID validates driver ID format.
func ValidateDriverID(driverID string) error {
	if driverID == "" {
		return fmt.Errorf("driver ID is required")
	}
	// Driver IDs typically follow pattern like "driver-123"
	matched, _ := regexp.MatchString(`^driver-\d+$`, driverID)
	if !matched {
		return fmt.Errorf("invalid driver ID format: %s", driverID)
	}
	return nil
}

// ValidateAssignmentID validates assignment ID format.
func ValidateAssignmentID(assignmentID string) error {
	if assignmentID == "" {
		return fmt.Errorf("assignment ID is required")
	}
	matched, _ := regexp.MatchString(`^assignment-\d+$`, assignmentID)
	if !matched {
		return fmt.Errorf("invalid assignment ID format: %s", assignmentID)
	}
	return nil
}