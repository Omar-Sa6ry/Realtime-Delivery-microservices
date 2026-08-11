package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestContextHandler(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, nil)
	handler := NewContextHandler(baseHandler)
	logger := slog.New(handler)

	ctx := context.Background()
	ctx = WithTraceID(ctx, "test-trace-123")
	ctx = WithUserID(ctx, "test-user-456")
	ctx = WithLogContext(ctx, LogContext{
		Method: "POST",
		Path:   "/users",
	})

	logger.InfoContext(ctx, "test message")

	var logRecord map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logRecord); err != nil {
		t.Fatalf("failed to unmarshal log JSON: %v", err)
	}

	if logRecord["msg"] != "test message" {
		t.Errorf("expected msg to be 'test message', got '%v'", logRecord["msg"])
	}
	if logRecord["traceId"] != "test-trace-123" {
		t.Errorf("expected traceId to be 'test-trace-123', got '%v'", logRecord["traceId"])
	}
	if logRecord["userId"] != "test-user-456" {
		t.Errorf("expected userId to be 'test-user-456', got '%v'", logRecord["userId"])
	}
	if logRecord["method"] != "POST" {
		t.Errorf("expected method to be 'POST', got '%v'", logRecord["method"])
	}
	if logRecord["path"] != "/users" {
		t.Errorf("expected path to be '/users', got '%v'", logRecord["path"])
	}
}
