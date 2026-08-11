package logging

import (
	"context"
)

type logContextKey string

const (
	TraceIDKey logContextKey = "trace_id"
	UserIDKey  logContextKey = "user_id"
	MethodKey  logContextKey = "method"
	PathKey    logContextKey = "path"
)

// LogContext holds logging context fields
type LogContext struct {
	TraceID string
	UserID  string
	Method  string
	Path    string
}

// WithLogContext returns a new context with the provided LogContext fields populated
func WithLogContext(ctx context.Context, fields LogContext) context.Context {
	if fields.TraceID != "" {
		ctx = context.WithValue(ctx, TraceIDKey, fields.TraceID)
	}
	if fields.UserID != "" {
		ctx = context.WithValue(ctx, UserIDKey, fields.UserID)
	}
	if fields.Method != "" {
		ctx = context.WithValue(ctx, MethodKey, fields.Method)
	}
	if fields.Path != "" {
		ctx = context.WithValue(ctx, PathKey, fields.Path)
	}
	return ctx
}

// WithTraceID returns a new context with the trace ID populated
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// WithUserID returns a new context with the user ID populated
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// GetTraceID retrieves the trace ID from the context
func GetTraceID(ctx context.Context) string {
	if val, ok := ctx.Value(TraceIDKey).(string); ok {
		return val
	}
	// Fallback to check other potential keys (e.g. from grpc interceptors)
	if val, ok := ctx.Value("x-correlation-id").(string); ok {
		return val
	}
	return ""
}

// GetUserID retrieves the user ID from the context
func GetUserID(ctx context.Context) string {
	if val, ok := ctx.Value(UserIDKey).(string); ok {
		return val
	}
	if val, ok := ctx.Value("x-user-id").(string); ok {
		return val
	}
	return ""
}

// GetMethod retrieves the method from the context
func GetMethod(ctx context.Context) string {
	if val, ok := ctx.Value(MethodKey).(string); ok {
		return val
	}
	return ""
}

// GetPath retrieves the path from the context
func GetPath(ctx context.Context) string {
	if val, ok := ctx.Value(PathKey).(string); ok {
		return val
	}
	return ""
}
