package websocket

import (
	"encoding/json"
	"testing"
)

func TestWsExceptionError(t *testing.T) {
	e := NewWsException(WsErrorForbidden, "", false)
	if e.Error() != string(WsErrorForbidden) {
		t.Errorf("expected message to default to code, got %s", e.Error())
	}

	e = NewWsException(WsErrorNotFound, "Delivery not found", false)
	if e.Error() != "Delivery not found" {
		t.Errorf("expected custom message, got %s", e.Error())
	}
}

func TestWsExceptionPayload(t *testing.T) {
	e := NewWsException(WsErrorRateLimited, "Slow down", true)
	payload := e.Payload("req-42")

	if payload.Code != WsErrorRateLimited {
		t.Errorf("expected code RATE_LIMITED, got %s", payload.Code)
	}
	if payload.Message != "Slow down" {
		t.Errorf("expected message 'Slow down', got %s", payload.Message)
	}
	if !payload.Retryable {
		t.Error("expected retryable to be true")
	}
	if payload.RequestID != "req-42" {
		t.Errorf("expected requestId req-42, got %s", payload.RequestID)
	}
}

func TestBuildErrorEnvelope(t *testing.T) {
	raw, err := BuildErrorEnvelope(WsErrorUnauthenticated, "Missing token", "req-1", false)
	if err != nil {
		t.Fatalf("BuildErrorEnvelope returned error: %v", err)
	}

	var envelope struct {
		Type      ServerMessageType `json:"type"`
		Timestamp string            `json:"timestamp"`
		RequestID string            `json:"requestId"`
		Data      WsErrorPayload    `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("failed to unmarshal envelope: %v", err)
	}

	if envelope.Type != ServerMessageError {
		t.Errorf("expected type ERROR, got %s", envelope.Type)
	}
	if envelope.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
	if envelope.RequestID != "req-1" {
		t.Errorf("expected requestId req-1, got %s", envelope.RequestID)
	}
	if envelope.Data.Code != WsErrorUnauthenticated {
		t.Errorf("expected data.code UNAUTHENTICATED, got %s", envelope.Data.Code)
	}
}

func TestTerminalCloseCode(t *testing.T) {
	tests := []struct {
		code     WsErrorCode
		closeCode int
		ok       bool
	}{
		{WsErrorUnauthenticated, WsCloseUnauthorized, true},
		{WsErrorForbidden, WsCloseForbidden, true},
		{WsErrorRateLimited, WsCloseRateLimited, true},
		{WsErrorTooLarge, WsClosePayloadTooLarge, true},
		{WsErrorNotFound, 0, false},
		{WsErrorInternal, 0, false},
	}

	for _, tt := range tests {
		got, ok := terminalCloseCode(tt.code)
		if got != tt.closeCode || ok != tt.ok {
			t.Errorf("terminalCloseCode(%s) = (%d, %v), want (%d, %v)", tt.code, got, ok, tt.closeCode, tt.ok)
		}
	}
}

func TestWsExceptionFilter(t *testing.T) {
	filter := NewWsExceptionFilter()

	t.Run("terminal error closes socket", func(t *testing.T) {
		conn := &fakeConn{}
		filter.Handle(conn, NewWsException(WsErrorUnauthenticated, "Missing token", false), "req-9")

		if conn.written == nil {
			t.Fatal("expected an ERROR envelope to be written")
		}
		var envelope struct {
			Type ServerMessageType `json:"type"`
		}
		if err := json.Unmarshal(conn.written, &envelope); err != nil {
			t.Fatalf("failed to unmarshal written envelope: %v", err)
		}
		if envelope.Type != ServerMessageError {
			t.Errorf("expected type ERROR, got %s", envelope.Type)
		}
		if conn.closeCode != WsCloseUnauthorized {
			t.Errorf("expected close code %d, got %d", WsCloseUnauthorized, conn.closeCode)
		}
	})

	t.Run("non-terminal error does not close socket", func(t *testing.T) {
		conn := &fakeConn{}
		filter.Handle(conn, NewWsException(WsErrorNotFound, "Delivery not found", false), "")

		if conn.written == nil {
			t.Fatal("expected an ERROR envelope to be written")
		}
		if conn.closeCode != 0 {
			t.Errorf("expected no close, got close code %d", conn.closeCode)
		}
	})

	t.Run("nil arguments are ignored", func(t *testing.T) {
		filter.Handle(nil, NewWsException(WsErrorInternal, "boom", true), "")
		filter.Handle(&fakeConn{}, nil, "")
	})
}

type fakeConn struct {
	written   []byte
	closeCode int
}

func (c *fakeConn) WriteText(data []byte) error {
	c.written = append([]byte(nil), data...)
	return nil
}

func (c *fakeConn) CloseWith(code int, reason string) error {
	c.closeCode = code
	return nil
}