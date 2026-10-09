package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

type ContextHandler struct {
	slog.Handler
}

func NewContextHandler(target slog.Handler) *ContextHandler {
	return &ContextHandler{Handler: target}
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx == nil {
		return h.Handler.Handle(ctx, r)
	}

	if traceID := GetTraceID(ctx); traceID != "" {
		r.AddAttrs(slog.String("traceId", traceID))
	}
	if userID := GetUserID(ctx); userID != "" {
		r.AddAttrs(slog.String("userId", userID))
	}
	if method := GetMethod(ctx); method != "" {
		r.AddAttrs(slog.String("method", method))
	}
	if path := GetPath(ctx); path != "" {
		r.AddAttrs(slog.String("path", path))
	}

	return h.Handler.Handle(ctx, r)
}

type DevHandler struct {
	w io.Writer
}

func NewDevHandler(w io.Writer) *DevHandler {
	if w == nil {
		w = os.Stdout
	}
	return &DevHandler{w: w}
}

func (h *DevHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= ParseLevel(os.Getenv("LOG_LEVEL"))
}

func (h *DevHandler) Handle(ctx context.Context, r slog.Record) error {
	tStr := r.Time.Format(time.RFC3339)

	var color string
	var levelStr string
	switch r.Level {
	case slog.LevelDebug:
		color = "\x1b[35m" // Magenta
		levelStr = "DEBUG"
	case slog.LevelInfo:
		color = "\x1b[32m" // Green (using green for info is very readable in go console)
		levelStr = "INFO"
	case slog.LevelWarn:
		color = "\x1b[33m" // Yellow
		levelStr = "WARN"
	case slog.LevelError:
		color = "\x1b[31m" // Red
		levelStr = "ERROR"
	default:
		color = "\x1b[0m"
		levelStr = r.Level.String()
	}

	// Trace Info
	traceInfo := ""
	if traceID := GetTraceID(ctx); traceID != "" {
		traceInfo = fmt.Sprintf(" [TraceID: %s]", traceID)
	}

	// Context name
	contextVal := "Application"
	extraAttrs := make([]slog.Attr, 0, r.NumAttrs())
	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "context" {
			contextVal = attr.Value.String()
		} else {
			extraAttrs = append(extraAttrs, attr)
		}
		return true
	})

	contextInfo := fmt.Sprintf(" [%s]", contextVal)

	// Contextual fields
	contextFields := ""
	if userID := GetUserID(ctx); userID != "" {
		contextFields += fmt.Sprintf(" userId=%s", userID)
	}
	if method := GetMethod(ctx); method != "" {
		contextFields += fmt.Sprintf(" method=%s", method)
	}
	if path := GetPath(ctx); path != "" {
		contextFields += fmt.Sprintf(" path=%s", path)
	}

	// Log extra attributes if any
	attrsStr := ""
	for _, attr := range extraAttrs {
		attrsStr += fmt.Sprintf(" %s=%v", attr.Key, attr.Value.Any())
	}

	// Format matching NestJS structured logger style
	fmt.Fprintf(h.w, "%s[%s] %s%s%s: %s%s%s\x1b[0m\n", color, levelStr, tStr, traceInfo, contextInfo, r.Message, contextFields, attrsStr)
	return nil
}

func (h *DevHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Basic dev handler does not carry attributes on grouping, returning as is
	return h
}

func (h *DevHandler) WithGroup(name string) slog.Handler {
	return h
}

func InitLogger() *slog.Logger {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = os.Getenv("NODE_ENV")
	}

	var baseHandler slog.Handler
	if env == "production" {
		baseHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: ParseLevel(os.Getenv("LOG_LEVEL")),
		})
	} else {
		baseHandler = NewDevHandler(os.Stdout)
	}

	logger := slog.New(NewContextHandler(baseHandler))
	slog.SetDefault(logger)
	return logger
}

func ParseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "err":
		return slog.LevelError
	case "info", "":
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}
