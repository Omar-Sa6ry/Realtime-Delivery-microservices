package domain

import (
	"fmt"
)

func ValidateDriverTransition(from, to DriverStatus, action string) error {
	if !IsValidDriverTransition(from, to, action) {
		return fmt.Errorf("invalid driver transition: from %s to %s via %s", from, to, action)
	}
	return nil
}

func ValidateAssignmentTransition(from, to AssignmentStatus, action string) error {
	if !IsValidAssignmentTransition(from, to, action) {
		return fmt.Errorf("invalid assignment transition: from %s to %s via %s", from, to, action)
	}
	return nil
}

func ValidateTransition(aggregateType string, from, to interface{}, action string) error {
	switch aggregateType {
	case "driver":
		driverFrom, ok := from.(DriverStatus)
		if !ok {
			return fmt.Errorf("invalid driver status type: %T", from)
		}
		driverTo, ok := to.(DriverStatus)
		if !ok {
			return fmt.Errorf("invalid driver status type: %T", to)
		}
		return ValidateDriverTransition(driverFrom, driverTo, action)

	case "assignment":
		assignmentFrom, ok := from.(AssignmentStatus)
		if !ok {
			return fmt.Errorf("invalid assignment status type: %T", from)
		}
		assignmentTo, ok := to.(AssignmentStatus)
		if !ok {
			return fmt.Errorf("invalid assignment status type: %T", to)
		}
		return ValidateAssignmentTransition(assignmentFrom, assignmentTo, action)

	default:
		return fmt.Errorf("unsupported aggregate type: %s", aggregateType)
	}
}

var ValidateTransitionError = &Error{Code: "validate_transition_error", Message: "transition validation failed"}

func IsValidDriverStatusTransition(from, to DriverStatus, action string) bool {
	transitions, ok := DriverTransitionMap[from]
	if !ok {
		return false
	}
	for _, t := range transitions {
		if t.From == from && t.Action == action && t.To == to {
			return true
		}
	}
	return false
}

func IsValidAssignmentStatusTransition(from, to AssignmentStatus, action string) bool {
	transitions, ok := AssignmentTransitionMap[from]
	if !ok {
		return false
	}
	for _, t := range transitions {
		if t.From == from && t.Action == action && t.To == to {
			return true
		}
	}
	return false
}