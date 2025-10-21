package logger

import (
	"context"
	"log/slog"
)

type contextKey int

const LoggerKey contextKey = iota

// WithLogger insert logger into context
func WithLogger(ctx context.Context, lg *slog.Logger) context.Context {
	return context.WithValue(ctx, LoggerKey, lg)
}

// FromContext return logger from context or slog.Default()
func FromContext(ctx context.Context) *slog.Logger {
	var lgAny any

	if ctx != nil {
		lgAny = ctx.Value(LoggerKey)
	}

	if lg, ok := lgAny.(*slog.Logger); ok && lg != nil {
		return lg
	}

	return slog.Default()
}

// FromContextOrNop return logger from context or NopLogger
func FromContextOrNop(ctx context.Context) *slog.Logger {
	var lgAny any

	if ctx != nil {
		lgAny = ctx.Value(LoggerKey)
	}

	if lg, ok := lgAny.(*slog.Logger); ok && lg != nil {
		return lg
	}

	return NewNopLogger()
}
