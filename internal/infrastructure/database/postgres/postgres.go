package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"
)

func Open(cfg conf.DatabaseConfig) (*database.Dao, error) {
	db, err := sql.OpenPgSql(&sql.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		DbName:   cfg.DBName,
	})
	if err != nil {
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
