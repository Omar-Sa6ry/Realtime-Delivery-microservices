package websocket

import (
	"encoding/json"
	"time"
)

// WsErrorCode is the canonical error code sent to clients —
// matches the TypeScript WsErrorCode enum.
type WsErrorCode string

const (
	WsErrorUnauthenticated    WsErrorCode = "UNAUTHENTICATED"
	WsErrorForbidden          WsErrorCode = "FORBIDDEN"
	WsErrorInvalidMessage     WsErrorCode = "INVALID_MESSAGE"
	WsErrorInvalidDeliveryID  WsErrorCode = "INVALID_DELIVERY_ID"
	WsErrorNotFound           WsErrorCode = "NOT_FOUND"
	WsErrorRateLimited        WsErrorCode = "RATE_LIMITED"
	WsErrorTooLarge           WsErrorCode = "TOO_LARGE"
	WsErrorStaleCommand       WsErrorCode = "STALE_COMMAND"
	WsErrorServiceUnavailable WsErrorCode = "SERVICE_UNAVAILABLE"
	WsErrorInternal           WsErrorCode = "INTERNAL_ERROR"
)

// WsErrorPayload is the error envelope sent over the wire —
// matches the TypeScript WsErrorPayload interface.
type WsErrorPayload struct {
	Code      WsErrorCode `json:"code"`
	Message   string      `json:"message"`
	Retryable bool        `json:"retryable"`
	RequestID string      `json:"requestId,omitempty"`
}

// WsException is a WebSocket protocol error —
// matches the TypeScript WsException class.
type WsException struct {
	Code      WsErrorCode
	Message   string
	Retryable bool
}

// NewWsException creates a WsException, defaulting the message to the code.
func NewWsException(code WsErrorCode, message string, retryable bool) *WsException {
	if message == "" {
		message = string(code)
	}
	return &WsException{Code: code, Message: message, Retryable: retryable}
}

// Error implements the error interface.
func (e *WsException) Error() string {
	return e.Message
}

// Payload converts the exception into the wire error envelope —
// matches the TS WsException.toPayload().
func (e *WsException) Payload(requestID string) WsErrorPayload {
	return WsErrorPayload{
		Code:      e.Code,
		Message:   e.Message,
		Retryable: e.Retryable,
		RequestID: requestID,
	}
}

// WsCloseCode is the WebSocket close code for terminal errors —
// matches the TypeScript WsCloseCode enum.
const (
	WsCloseUnauthorized    = 4401
	WsCloseForbidden       = 4403
	WsCloseRateLimited     = 4408
	WsClosePayloadTooLarge = 4413
	WsCloseInternalError   = 4500
)

// errorEnvelope is the wire ERROR envelope — matches the envelope built by the
// TS WsExceptionFilter / RealtimeWsAdapter.errorEnvelope().
type errorEnvelope struct {
	Type      ServerMessageType `json:"type"`
	Timestamp string            `json:"timestamp"`
	RequestID string            `json:"requestId,omitempty"`
	Data      WsErrorPayload    `json:"data"`
}

// BuildErrorEnvelope serialises an error into the standard wire ERROR envelope.
func BuildErrorEnvelope(code WsErrorCode, message string, requestID string, retryable bool) ([]byte, error) {
	envelope := errorEnvelope{
		Type:      ServerMessageError,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: requestID,
		Data: WsErrorPayload{
			Code:      code,
			Message:   message,
			Retryable: retryable,
			RequestID: requestID,
		},
	}
	return json.Marshal(envelope)
}

// terminalCloseCode maps an error code to a terminal WebSocket close code.
// Returns ok=false for non-terminal codes — mirrors the TS WsExceptionFilter switch.
func terminalCloseCode(code WsErrorCode) (int, bool) {
	switch code {
	case WsErrorUnauthenticated:
		return WsCloseUnauthorized, true
	case WsErrorForbidden:
		return WsCloseForbidden, true
	case WsErrorRateLimited:
		return WsCloseRateLimited, true
	case WsErrorTooLarge:
		return WsClosePayloadTooLarge, true
	default:
		return 0, false
	}
}