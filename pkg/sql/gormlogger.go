package sql

import (
	"log/slog"
	"time"

	pkglog "github.com/gonotelm-lab/gonotelm/pkg/log"
	gormlogger "gorm.io/gorm/logger"
)

func NewSlogGormLogger(logger *slog.Logger) gormlogger.Interface {
	if logger == nil {
		logger = slog.Default()
	}

	return gormlogger.NewSlogLogger(
		logger.With(pkglog.AttrKeyComponent, pkglog.ComponentGorm),
		gormlogger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		},
	)
}
