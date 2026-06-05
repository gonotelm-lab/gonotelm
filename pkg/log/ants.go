package log

import (
	"log/slog"

	"github.com/panjf2000/ants/v2"
)

type AntsLogger struct{}

var _ ants.Logger = &AntsLogger{}

func (l *AntsLogger) Printf(format string, args ...any) {
	slog.Info(format, args...)
}
