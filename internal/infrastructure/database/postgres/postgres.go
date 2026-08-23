package postgres

import (
	"context"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/plugin/opentelemetry/tracing"
)

func Open(cfg *sql.Config) (*database.Dao, error) {
	db, err := sql.OpenPgSql(cfg)
	if err != nil {
		return nil, err
	}

	// add opentelemetry tracing
	// 需要和pkgtrace.Init配合处理 因为pkgtrace.Init中注入了全局的Provider等内容
	if err := db.Use(tracing.NewPlugin(
		tracing.WithoutQueryVariables(),
		tracing.WithQueryFormatter(strings.TrimSpace),
	)); err != nil {
		return nil, err
	}

	closer := misc.CloserFunc(func(_ context.Context) error {
		if sqlDb, err := db.DB(); err == nil {
			return sqlDb.Close()
		}
		return nil
	})

	return database.NewDao(
		closer,
		NewNotebookStoreImpl(db),
		NewSourceStoreImpl(db),
		NewChatStoreImpl(db),
		NewChatMessageStoreImpl(db),
		NewArtifactStoreImpl(db),
		NewWorkerCheckpointStoreImpl(db),
	), nil
}
