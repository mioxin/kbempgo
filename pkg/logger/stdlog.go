package logger

import (
	"context"
	"fmt"
	"log/slog"
)

// Printer many libraries have Logger interface like that one
type Printer interface {
	Printf(format string, args ...any)
}

// Printer many libraries have Logger interface like that one
type CtxPrinter interface {
	Printf(ctx context.Context, format string, args ...any)
}

type SlogPrinter struct {
	*slog.Logger

	Level slog.Level
}

func (s *SlogPrinter) Printf(format string, args ...any) {
	s.Log(context.Background(), s.Level, fmt.Sprintf(format, args...))
}

type SlogCtxPrinter struct {
	*slog.Logger

	Level slog.Level
}

func (s *SlogCtxPrinter) Printf(ctx context.Context, format string, args ...any) {
	s.Log(ctx, s.Level, fmt.Sprintf(format, args...))
}

func NewPrinterAt(lg *slog.Logger, lvl slog.Level) Printer {
	return &SlogPrinter{
		Logger: lg,
		Level:  lvl,
	}
}

func NewCtxPrinterAt(lg *slog.Logger, lvl slog.Level) CtxPrinter {
	return &SlogCtxPrinter{
		Logger: lg,
		Level:  lvl,
	}
}
