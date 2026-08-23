package logger

import (
	"context"
	"log/slog"
	"os"
)

var defaultLogger *slog.Logger

func init() {
	// Initialize default structured JSON handler compatible with Google Cloud Logging
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Map slog Level to GCP Cloud Logging severity field
			if a.Key == slog.LevelKey {
				a.Key = "severity"
			}
			return a
		},
	})
	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

// Get returns the configured global structured JSON logger.
func Get() *slog.Logger {
	return defaultLogger
}

// Info logs an informational message with structured key-value attributes.
func Info(ctx context.Context, msg string, args ...any) {
	defaultLogger.InfoContext(ctx, msg, args...)
}

// Warn logs a warning message with structured key-value attributes.
func Warn(ctx context.Context, msg string, args ...any) {
	defaultLogger.WarnContext(ctx, msg, args...)
}

// Error logs an error message with structured key-value attributes.
func Error(ctx context.Context, msg string, args ...any) {
	defaultLogger.ErrorContext(ctx, msg, args...)
}

// Debug logs a debug message with structured key-value attributes.
func Debug(ctx context.Context, msg string, args ...any) {
	defaultLogger.DebugContext(ctx, msg, args...)
}

// WithComponent creates a child logger with a component attribute.
func WithComponent(name string) *slog.Logger {
	return defaultLogger.With("component", name)
}
