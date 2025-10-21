package logger

import (
	"context"
	"log/slog"
)

type NopHandler struct {
}

func (NopHandler) Enabled(context.Context, slog.Level) bool {
	return false
}

func (NopHandler) Handle(context.Context, slog.Record) error {
	return nil
}

func (s *NopHandler) WithAttrs([]slog.Attr) slog.Handler {
	return s
}

func (s *NopHandler) WithGroup(string) slog.Handler {
	return s
}

func NewNopHandler() slog.Handler {
	return &NopHandler{}
}

func NewNopLogger() *slog.Logger {
	return slog.New(NewNopHandler())
}
