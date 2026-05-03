package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	hslog "github.com/hertz-contrib/logger/slog"
)

type hertzSlogger struct {
	base   *hslog.Logger
	logger *slog.Logger
	mu     sync.RWMutex
}

var _ hlog.FullLogger = (*hertzSlogger)(nil)

func newHertzSlogger(level *slog.LevelVar, output io.Writer) *hertzSlogger {
	if level == nil {
		level = &slog.LevelVar{}
		level.Set(slog.LevelInfo)
	}
	if output == nil {
		output = os.Stdout
	}

	handlerOptions := &slog.HandlerOptions{
		Level:       level,
		AddSource:   true,
		ReplaceAttr: replaceSourceFileWithBaseName,
	}
	base := hslog.NewLogger(
		hslog.WithOutput(output),
		hslog.WithLevel(level),
		hslog.WithHandlerOptions(handlerOptions),
	)

	return &hertzSlogger{
		base:   base,
		logger: base.Logger().With(AttrKeyComponent, ComponentHertz),
	}
}

func (l *hertzSlogger) log(level hlog.Level, v ...any) {
	l.logMessage(context.Background(), level, fmt.Sprint(v...))
}

func (l *hertzSlogger) logf(level hlog.Level, format string, v ...any) {
	l.logMessage(context.Background(), level, fmt.Sprintf(format, v...))
}

func (l *hertzSlogger) ctxLogf(level hlog.Level, ctx context.Context, format string, v ...any) {
	l.logMessage(ctx, level, fmt.Sprintf(format, v...))
}

func (l *hertzSlogger) logMessage(ctx context.Context, level hlog.Level, msg string) {
	if ctx == nil {
		ctx = context.Background()
	}

	l.mu.RLock()
	logger := l.logger
	l.mu.RUnlock()

	logger.Log(ctx, hertzLevelToSlogLevel(level), msg)
}

func (l *hertzSlogger) Trace(v ...any) {
	l.log(hlog.LevelTrace, v...)
}

func (l *hertzSlogger) Debug(v ...any) {
	l.log(hlog.LevelDebug, v...)
}

func (l *hertzSlogger) Info(v ...any) {
	l.log(hlog.LevelInfo, v...)
}

func (l *hertzSlogger) Notice(v ...any) {
	l.log(hlog.LevelNotice, v...)
}

func (l *hertzSlogger) Warn(v ...any) {
	l.log(hlog.LevelWarn, v...)
}

func (l *hertzSlogger) Error(v ...any) {
	l.log(hlog.LevelError, v...)
}

func (l *hertzSlogger) Fatal(v ...any) {
	l.log(hlog.LevelFatal, v...)
}

func (l *hertzSlogger) Tracef(format string, v ...any) {
	l.logf(hlog.LevelTrace, format, v...)
}

func (l *hertzSlogger) Debugf(format string, v ...any) {
	l.logf(hlog.LevelDebug, format, v...)
}

func (l *hertzSlogger) Infof(format string, v ...any) {
	l.logf(hlog.LevelInfo, format, v...)
}

func (l *hertzSlogger) Noticef(format string, v ...any) {
	l.logf(hlog.LevelNotice, format, v...)
}

func (l *hertzSlogger) Warnf(format string, v ...any) {
	l.logf(hlog.LevelWarn, format, v...)
}

func (l *hertzSlogger) Errorf(format string, v ...any) {
	l.logf(hlog.LevelError, format, v...)
}

func (l *hertzSlogger) Fatalf(format string, v ...any) {
	l.logf(hlog.LevelFatal, format, v...)
}

func (l *hertzSlogger) CtxTracef(ctx context.Context, format string, v ...any) {
	l.ctxLogf(hlog.LevelTrace, ctx, format, v...)
}

func (l *hertzSlogger) CtxDebugf(ctx context.Context, format string, v ...any) {
	l.ctxLogf(hlog.LevelDebug, ctx, format, v...)
}

func (l *hertzSlogger) CtxInfof(ctx context.Context, format string, v ...any) {
	l.ctxLogf(hlog.LevelInfo, ctx, format, v...)
}

func (l *hertzSlogger) CtxNoticef(ctx context.Context, format string, v ...any) {
	l.ctxLogf(hlog.LevelNotice, ctx, format, v...)
}

func (l *hertzSlogger) CtxWarnf(ctx context.Context, format string, v ...any) {
	l.ctxLogf(hlog.LevelWarn, ctx, format, v...)
}

func (l *hertzSlogger) CtxErrorf(ctx context.Context, format string, v ...any) {
	l.ctxLogf(hlog.LevelError, ctx, format, v...)
}

func (l *hertzSlogger) CtxFatalf(ctx context.Context, format string, v ...any) {
	l.ctxLogf(hlog.LevelFatal, ctx, format, v...)
}

func (l *hertzSlogger) SetLevel(level hlog.Level) {
	l.base.SetLevel(level)
}

func (l *hertzSlogger) SetOutput(writer io.Writer) {
	if writer == nil {
		writer = os.Stdout
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.base.SetOutput(writer)
	l.logger = l.base.Logger().With(AttrKeyComponent, ComponentHertz)
}

func hertzLevelToSlogLevel(level hlog.Level) slog.Level {
	switch level {
	case hlog.LevelTrace:
		return hslog.LevelTrace
	case hlog.LevelDebug:
		return slog.LevelDebug
	case hlog.LevelInfo:
		return slog.LevelInfo
	case hlog.LevelNotice:
		return hslog.LevelNotice
	case hlog.LevelWarn:
		return slog.LevelWarn
	case hlog.LevelError:
		return slog.LevelError
	case hlog.LevelFatal:
		return hslog.LevelFatal
	default:
		return slog.LevelWarn
	}
}
