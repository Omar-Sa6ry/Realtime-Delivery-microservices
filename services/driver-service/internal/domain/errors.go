package domain

import "fmt"

var (
	ErrDriverNotFound          = &Error{Code: "driver_not_found", Message: "driver not found"}
	ErrDriverNotAvailable      = &Error{Code: "driver_not_available", Message: "driver is not available for assignment"}
	ErrDriverAlreadyOnline     = &Error{Code: "driver_already_online", Message: "driver is already online"}
	ErrDriverAlreadyOffline    = &Error{Code: "driver_already_offline", Message: "driver is already offline"}
	ErrDriverNotOffline        = &Error{Code: "driver_not_offline", Message: "driver is not offline"}
	ErrDriverNotAvailableForAssignment = &Error{Code: "driver_not_available_for_assignment", Message: "driver is not in a state available for assignment"}
	ErrAssignmentNotFound      = &Error{Code: "assignment_not_found", Message: "assignment not found"}
	ErrAssignmentInvalidState  = &Error{Code: "assignment_invalid_state", Message: "assignment is not in a valid state for this operation"}
	ErrAssignmentAlreadyAccepted = &Error{Code: "assignment_already_accepted", Message: "assignment already accepted"}
	ErrAssignmentAlreadyRejected = &Error{Code: "assignment_already_rejected", Message: "assignment already rejected"}
	ErrAssignmentExpired       = &Error{Code: "assignment_expired", Message: "assignment has expired"}
	ErrAssignmentCannotCancel  = &Error{Code: "assignment_cannot_cancel", Message: "assignment cannot be cancelled in current state"}
	ErrDriverAlreadyReserved   = &Error{Code: "driver_already_reserved", Message: "driver is already reserved"}
	ErrInvalidTransition       = &Error{Code: "invalid_transition", Message: "invalid state transition"}
	ErrInvalidArgument         = &Error{Code: "invalid_argument", Message: "invalid argument"}
	ErrInternal                = &Error{Code: "internal_error", Message: "internal error"}
)

// Error represents a domain error with a code and message.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Is checks if the error matches the given code.
func (e *Error) IsCode(code string) bool {
	return e.Code == code
}

// Unwrap implements the builtin error interface.
func (e *Error) Unwrap() error {
	return e
}