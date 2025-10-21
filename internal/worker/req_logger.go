package worker

import (
	"fmt"
	"log/slog"
)

type ReqLogger struct {
	slog.Logger
}

func (l *ReqLogger) Errorf(format string, v ...any) {
	l.Error(fmt.Sprintf(format, v...))
}
func (l *ReqLogger) Warnf(format string, v ...any) {
	l.Warn(fmt.Sprintf(format, v...))
}
func (l *ReqLogger) Debugf(format string, v ...any) {
	l.Debug(fmt.Sprintf(format, v...))
}

// func NewReqLogger(l *slog.Logger) *ReqLogger {
// 	return &ReqLogger{Lg: l}
// }
