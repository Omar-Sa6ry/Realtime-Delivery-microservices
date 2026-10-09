package logging

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"":         slog.LevelInfo,
		"debug":    slog.LevelDebug,
		"DEBUG":    slog.LevelDebug,
		"info":     slog.LevelInfo,
		" warn ":   slog.LevelWarn,
		"warning":  slog.LevelWarn,
		"error":    slog.LevelError,
		"nonsense": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Fatalf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

// captureStdout swaps os.Stdout for a pipe while fn runs and returns everything
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = old }()

	fn()

	writer.Close()
	os.Stdout = old
	out, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return string(out)
}

func TestInitLoggerRespectsLogLevel(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("LOG_LEVEL", "error")

	out := captureStdout(t, func() {
		logger := InitLogger()
		logger.Info("should be filtered out")
		logger.Error("boom")
	})

	if strings.Contains(out, "should be filtered out") {
		t.Fatalf("info record leaked past LOG_LEVEL=error: %s", out)
	}
	if !strings.Contains(out, `"msg":"boom"`) {
		t.Fatalf("expected json error record, got %q", out)
	}
}

func TestStdlibBridgeRoutesThroughSlog(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("LOG_LEVEL", "debug")

	logOutput := captureStdout(t, func() {
		InitLoggerWithStdlibBridge()

		log.Println("plain startup message")
		log.Printf("WARNING: mongodb ping failed: %v", context.DeadlineExceeded)
		log.Printf("failed to dial user-service: %v", context.Canceled)
	})

	if !strings.Contains(logOutput, `"msg":"plain startup message"`) {
		t.Fatalf("plain message not structured: %q", logOutput)
	}
	if !strings.Contains(logOutput, `"msg":"WARNING: mongodb ping failed: context deadline exceeded"`) {
		t.Fatalf("warning not formatted or classified: %q", logOutput)
	}
	if !strings.Contains(logOutput, `"level":"ERROR"`) || !strings.Contains(logOutput, "failed to dial user-service") {
		t.Fatalf("failure message not classified as error: %q", logOutput)
	}
	for _, line := range strings.Split(strings.TrimSpace(logOutput), "\n") {
		if !strings.HasPrefix(line, "{") {
			t.Fatalf("non-json line leaked from stdlib bridge: %q", line)
		}
	}
}
