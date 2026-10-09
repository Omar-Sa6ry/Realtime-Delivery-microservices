package logging

import (
	"log"
	"log/slog"
	"strings"
)

type stdlibBridge struct{}

func (stdlibBridge) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\r\n")
	if msg == "" {
		return len(p), nil
	}

	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "fatal"), strings.Contains(lower, "error"), strings.Contains(lower, "failed"):
		slog.Error(msg)
	case strings.Contains(lower, "warning"), strings.Contains(lower, "warn"):
		slog.Warn(msg)
	default:
		slog.Info(msg)
	}
	return len(p), nil
}

func InitLoggerWithStdlibBridge() *slog.Logger {
	logger := InitLogger()
	log.SetFlags(0)
	log.SetOutput(stdlibBridge{})
	return logger
}
