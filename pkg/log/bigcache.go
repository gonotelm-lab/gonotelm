package log

import (
	"log/slog"

	"github.com/allegro/bigcache/v3"
)

type BigcacheLogger struct{}

var _ bigcache.Logger = &BigcacheLogger{}

func (l *BigcacheLogger) Printf(format string, v ...interface{}) {
	slog.Info(format, v...)
}
