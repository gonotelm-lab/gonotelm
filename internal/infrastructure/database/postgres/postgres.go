package postgres

import (
	"context"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"

	pg "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Open(cfg conf.DatabaseConfig) (*database.DAL, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)
	db, err := gorm.Open(pg.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	closer := misc.CloserFunc(func(_ context.Context) error {
		if sqlDb, err := db.DB(); err == nil {
			return sqlDb.Close()
		}
		return nil
	})

	return database.NewDAL(
		closer,
		NewNotebookStoreImpl(db),
		NewSourceStoreImpl(db),
		NewChatStoreImpl(db),
		NewChatMessageStoreImpl(db),
		NewArtifactTaskStoreImpl(db),
	), nil
}
