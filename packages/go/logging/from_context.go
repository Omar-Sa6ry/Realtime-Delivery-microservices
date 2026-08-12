package logging

import (
	"context"
	"log/slog"
)

// FromContext returns the logger enriched with any tracing context from ctx.
// It pulls traceId, userId from the context and returns a slog.Logger with those fields.
func FromContext(ctx context.Context) *slog.Logger {
	logger := slog.Default()
	if traceID := GetTraceID(ctx); traceID != "" {
		logger = logger.With("traceId", traceID)
	}
	if userID := GetUserID(ctx); userID != "" {
		logger = logger.With("userId", userID)
	}
	return logger
}
