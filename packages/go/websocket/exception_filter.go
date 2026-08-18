package websocket

import "log/slog"

// WsConn is the socket surface needed to send envelopes and close connections.
// Services adapt their WebSocket library to this interface —
// mirrors the TS client surface used by WsExceptionFilter.
type WsConn interface {
	// WriteText sends a text frame with the given JSON bytes.
	WriteText(data []byte) error
	// CloseWith closes the connection with the given WebSocket close code and reason.
	CloseWith(code int, reason string) error
}

// WsExceptionFilter converts WsException into the wire-protocol ERROR envelope
// and closes the socket for terminal error codes —
// mirrors the TS WsExceptionFilter class.
type WsExceptionFilter struct{}

// NewWsExceptionFilter creates an exception filter.
func NewWsExceptionFilter() *WsExceptionFilter {
	return &WsExceptionFilter{}
}

// Handle sends the ERROR envelope to the client and closes the socket for
// terminal error codes (UNAUTHENTICATED, FORBIDDEN, RATE_LIMITED, TOO_LARGE).
func (f *WsExceptionFilter) Handle(conn WsConn, exception *WsException, requestID string) {
	if conn == nil || exception == nil {
		return
	}

	envelope, err := BuildErrorEnvelope(exception.Code, exception.Message, requestID, exception.Retryable)
	if err != nil {
		slog.Warn("Failed to build WsException envelope", "error", err.Error())
		return
	}

	if err := conn.WriteText(envelope); err != nil {
		slog.Warn("Failed to send WsException envelope", "error", err.Error())
		return
	}

	if closeCode, ok := terminalCloseCode(exception.Code); ok {
		_ = conn.CloseWith(closeCode, string(exception.Code))
	}
}