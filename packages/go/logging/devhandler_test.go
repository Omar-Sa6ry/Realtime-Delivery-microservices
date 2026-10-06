package logging

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDevHandlerDefaultsToStdout(t *testing.T) {
	handler := NewDevHandler(nil)
	require.NotNil(t, handler)
}

func TestDevHandlerEnabled(t *testing.T) {
	handler := NewDevHandler(&bytes.Buffer{})

	t.Setenv("LOG_LEVEL", "debug")
	assert.True(t, handler.Enabled(context.Background(), slog.LevelDebug))

	t.Setenv("LOG_LEVEL", "DEBUG")
	assert.True(t, handler.Enabled(context.Background(), slog.LevelDebug))

	t.Setenv("LOG_LEVEL", "info")
	assert.False(t, handler.Enabled(context.Background(), slog.LevelDebug))
	assert.True(t, handler.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, handler.Enabled(context.Background(), slog.LevelError))
}

func TestDevHandlerFormatsEveryLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")

	tests := []struct {
		name      string
		level     slog.Level
		wantLabel string
	}{
		{"debug", slog.LevelDebug, "DEBUG"},
		{"info", slog.LevelInfo, "INFO"},
		{"warn", slog.LevelWarn, "WARN"},
		{"error", slog.LevelError, "ERROR"},
		{"unknown level", slog.Level(42), slog.Level(42).String()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(NewDevHandler(&buf))

			logger.Log(context.Background(), tt.level, "hello world")

			out := buf.String()
			assert.Contains(t, out, tt.wantLabel)
			assert.Contains(t, out, "hello world")
			assert.Contains(t, out, "\x1b[0m")
		})
	}
}

func TestDevHandlerRendersContextualFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewDevHandler(&buf))

	ctx := WithTraceID(context.Background(), "tr-1")
	ctx = WithUserID(ctx, "u-1")
	ctx = WithLogContext(ctx, LogContext{Method: "POST", Path: "/orders"})

	logger.InfoContext(ctx, "handled")

	out := buf.String()
	assert.Contains(t, out, "[TraceID: tr-1]")
	assert.Contains(t, out, "userId=u-1")
	assert.Contains(t, out, "method=POST")
	assert.Contains(t, out, "path=/orders")
}

func TestDevHandlerRendersExtraAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := NewDevHandler(&buf)
	logger := slog.New(handler)

	logger.Log(context.Background(), slog.LevelInfo, "msg", "custom", 7)

	out := buf.String()
	assert.Contains(t, out, "custom=7")
}

func TestDevHandlerAttributeOverridesGroupName(t *testing.T) {
	var buf bytes.Buffer
	handler := NewDevHandler(&buf)

	logger := slog.New(handler)
	logger.Log(context.Background(), slog.LevelInfo, "msg", "context", "Billing")

	assert.Contains(t, buf.String(), "[Billing]")
}

func TestDevHandlerWithAttrsAndWithGroupAreNoOps(t *testing.T) {
	handler := NewDevHandler(&bytes.Buffer{})

	assert.Same(t, handler, handler.WithAttrs([]slog.Attr{slog.String("a", "b")}))
	assert.Same(t, handler, handler.WithGroup("nested"))
}

func TestInitLoggerDevelopmentUsesDevHandler(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })

	t.Setenv("APP_ENV", "development")
	t.Setenv("NODE_ENV", "")

	logger := InitLogger()
	require.NotNil(t, logger)
	assert.Equal(t, logger, slog.Default())
}

func TestInitLoggerProductionUsesJSONHandler(t *testing.T) {
	previous := slog.Default()
	previousOut := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = previousOut
		slog.SetDefault(previous)
	})

	t.Setenv("APP_ENV", "production")
	t.Setenv("NODE_ENV", "")

	logger := InitLogger()
	require.NotNil(t, logger)

	logger.Info("production boot")

	require.NoError(t, w.Close())
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	assert.True(t, strings.Contains(buf.String(), `"msg":"production boot"`),
		"expected JSON log line, got %q", buf.String())
}
