package grpc_slog

import (
	"context"
	"log/slog"

	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
)

// InterceptorLogger adapts slog logger to interceptor logger.
func InterceptorLogger(lg *slog.Logger) grpc_logging.Logger {
	// grpc_logging signature almost the same as slog.Log, grpc_logger.Level match slog.Level
	// See:
	//   https://pkg.go.dev/log/slog#Level
	//   https://pkg.go.dev/github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging#Level
	return grpc_logging.LoggerFunc(func(ctx context.Context, lvl grpc_logging.Level, msg string, fields ...any) {
		// l := lg.WithOptions(zap.AddCallerSkip(1)).With(fields...)

		lg.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}
