package clickhouse

import (
	"context"
	"log/slog"

	pkglog "github.com/gonotelm-lab/gonotelm/pkg/log"
)

// DriverLogger returns a slog.Logger for clickhouse-go.
// The driver logs every query at debug level; we only forward warnings and errors.
func DriverLogger() *slog.Logger {
	return slog.New(&minLevelHandler{
		handler: pkglog.DefaultHandler(),
		level:   slog.LevelInfo,
	}).With(pkglog.AttrKeyComponent, pkglog.ComponentClickHouseGo)
}

type minLevelHandler struct {
	handler slog.Handler
	level   slog.Level
}

func (h *minLevelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level && h.handler.Enabled(ctx, level)
}

func (h *minLevelHandler) Handle(ctx context.Context, record slog.Record) error {
	return h.handler.Handle(ctx, record)
}

func (h *minLevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &minLevelHandler{
		handler: h.handler.WithAttrs(attrs),
		level:   h.level,
	}
}

func (h *minLevelHandler) WithGroup(name string) slog.Handler {
	return &minLevelHandler{
		handler: h.handler.WithGroup(name),
		level:   h.level,
	}
}
