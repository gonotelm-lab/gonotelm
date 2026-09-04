package log

import (
	"log/slog"

	"github.com/panjf2000/ants/v2"
)

type AntsLogger struct {
	logger *slog.Logger
}

var _ ants.Logger = &AntsLogger{}

func NewAntsLogger(logger *slog.Logger) *AntsLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return &AntsLogger{
		logger: logger.With(AttrKeyComponent, ComponentAnts),
	}
}

func (l *AntsLogger) Printf(format string, args ...any) {
	l.logger.Error(format, args...)
}
