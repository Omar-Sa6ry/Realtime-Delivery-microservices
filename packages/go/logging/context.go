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

type LogContext struct {
	TraceID string
	UserID  string
	Method  string
	Path    string
}

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

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

func GetTraceID(ctx context.Context) string {
	if val, ok := ctx.Value(TraceIDKey).(string); ok {
		return val
	}
	if val, ok := ctx.Value("x-correlation-id").(string); ok {
		return val
	}
	return ""
}

func GetUserID(ctx context.Context) string {
	if val, ok := ctx.Value(UserIDKey).(string); ok {
		return val
	}
	if val, ok := ctx.Value("x-user-id").(string); ok {
		return val
	}
	return ""
}

func GetMethod(ctx context.Context) string {
	if val, ok := ctx.Value(MethodKey).(string); ok {
		return val
	}
	return ""
}

func GetPath(ctx context.Context) string {
	if val, ok := ctx.Value(PathKey).(string); ok {
		return val
	}
	return ""
}
