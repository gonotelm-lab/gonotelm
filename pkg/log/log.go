package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	hslog "github.com/hertz-contrib/logger/slog"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

var (
	defaultLevelVar    = &slog.LevelVar{}
	defaultJSONHandler = newContextJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       defaultLevelVar,
		AddSource:   true,
		ReplaceAttr: replaceSourceFileWithBaseName,
	})
)

func init() {
	defaultLevelVar.Set(slog.LevelInfo)
}

func DefaultHandler() slog.Handler {
	return defaultJSONHandler
}

func InitDefault() *slog.Logger {
	logger := slog.New(DefaultHandler())
	slog.SetDefault(logger)
	return logger
}

func InitHertzDefault() {
	logger := newHertzSlogger(defaultLevelVar, os.Stdout)
	hlog.SetLogger(logger)
	hlog.SetLevel(toHertzLevel(defaultLevelVar.Level()))
}

func Init() {
	InitDefault()
	InitHertzDefault()
}

func SetLevel(level slog.Level) {
	defaultLevelVar.Set(level)
	hlog.SetLevel(toHertzLevel(level))
}

func SetLevelText(level string) error {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		SetLevel(slog.LevelInfo)
	case "trace":
		SetLevel(hslog.LevelTrace)
	case "debug":
		SetLevel(slog.LevelDebug)
	case "notice":
		SetLevel(hslog.LevelNotice)
	case "warn", "warning":
		SetLevel(slog.LevelWarn)
	case "error":
		SetLevel(slog.LevelError)
	case "fatal":
		SetLevel(hslog.LevelFatal)
	default:
		return fmt.Errorf("unsupported log level %q", level)
	}

	return nil
}

func Level() slog.Level {
	return defaultLevelVar.Level()
}

func toHertzLevel(level slog.Level) hlog.Level {
	switch {
	case level <= hslog.LevelTrace:
		return hlog.LevelTrace
	case level <= slog.LevelDebug:
		return hlog.LevelDebug
	case level <= slog.LevelInfo:
		return hlog.LevelInfo
	case level < slog.LevelWarn:
		return hlog.LevelNotice
	case level <= slog.LevelWarn:
		return hlog.LevelWarn
	case level <= slog.LevelError:
		return hlog.LevelError
	default:
		return hlog.LevelFatal
	}
}

type contextJSONHandler struct {
	handler slog.Handler
}

func newContextJSONHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	return &contextJSONHandler{
		handler: slog.NewJSONHandler(w, opts),
	}
}

func (h *contextJSONHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *contextJSONHandler) Handle(ctx context.Context, record slog.Record) error {
	if ctx != nil {
		if userID, ok := ctx.Value(pkgcontext.ContextKeyUserId).(string); ok && strings.TrimSpace(userID) != "" {
			record.AddAttrs(slog.String(AttrKeyUserID, userID))
		}
	}
	return h.handler.Handle(ctx, record)
}

func (h *contextJSONHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextJSONHandler{
		handler: h.handler.WithAttrs(attrs),
	}
}

func (h *contextJSONHandler) WithGroup(name string) slog.Handler {
	return &contextJSONHandler{
		handler: h.handler.WithGroup(name),
	}
}

func replaceSourceFileWithBaseName(groups []string, a slog.Attr) slog.Attr {
	if a.Key != slog.SourceKey {
		return a
	}

	src, ok := a.Value.Any().(*slog.Source)
	if !ok || src == nil {
		return a
	}

	copySource := *src
	copySource.File = filepath.Base(copySource.File)
	return slog.Any(slog.SourceKey, &copySource)
}
