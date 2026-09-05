package domain

import (
	"fmt"
)

// ValidateDriverTransition validates a driver state transition.
// It checks if the transition from 'from' to 'to' via the given 'action' is valid
// according to the driver state machine defined in driver.go.
func ValidateDriverTransition(from, to DriverStatus, action string) error {
	if !IsValidDriverTransition(from, to, action) {
		return fmt.Errorf("invalid driver transition: from %s to %s via %s", from, to, action)
	}
	return nil
}

// ValidateAssignmentTransition validates an assignment state transition.
// It checks if the transition from 'from' to 'to' via the given 'action' is valid
// according to the assignment state machine defined in assignment.go.
func ValidateAssignmentTransition(from, to AssignmentStatus, action string) error {
	if !IsValidAssignmentTransition(from, to, action) {
		return fmt.Errorf("invalid assignment transition: from %s to %s via %s", from, to, action)
	}
	return nil
}

// ValidateTransition is a unified validator that dispatches to the appropriate
// state machine validator based on the aggregate type and transition details.
// It supports both Driver and Assignment state transitions.
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

// ValidateTransitionError is returned when a transition validation fails.
var ValidateTransitionError = &Error{Code: "validate_transition_error", Message: "transition validation failed"}

// IsValidDriverStatusTransition checks if a driver status transition is valid.
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

// IsValidAssignmentStatusTransition checks if an assignment status transition is valid.
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