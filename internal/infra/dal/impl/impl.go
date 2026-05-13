package impl

import (
	"context"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/impl/postgres"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"
)

type Type string

const (
	Postgres Type = "postgres"
)

func New(t Type, cfg *sql.Config) (*dal.DAL, error) {
	switch t {
	case Postgres:
		db, err := sql.OpenPgSql(cfg)
		if err != nil {
			return nil, err
		}

		closer := misc.CloserFunc(func(_ context.Context) error {
			if sqlDb, err := db.DB(); err == nil {
				return sqlDb.Close()
			}
			return nil
		})

		return &dal.DAL{
			Closer: closer,

			NotebookStore:    postgres.NewNotebookStoreImpl(db),
			SourceStore:      postgres.NewSourceStoreImpl(db),
			ChatStore:        postgres.NewChatStoreImpl(db),
			ChatMessageStore: postgres.NewChatMessageStoreImpl(db),
		}, nil
	}

	return nil, fmt.Errorf("impl type %s is not supported", t)
}
