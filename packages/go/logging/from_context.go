package logging

import (
	"context"
	"log/slog"
)

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
