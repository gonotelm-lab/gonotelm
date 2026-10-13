package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"

	"gorm.io/gorm"
)

type TransactorImpl struct {
	db *gorm.DB
}

var _ database.Transactor = &TransactorImpl{}

func NewTransactorImpl(db *gorm.DB) *TransactorImpl {
	return &TransactorImpl{db: db}
}

func (t *TransactorImpl) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(database.WithTx(ctx, tx))
	})
}

// dbFromContext 优先用 ctx 里的事务句柄，没有则用连接池
func dbFromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := database.TxFromContext(ctx); ok {
		if gormTx, ok := tx.(*gorm.DB); ok {
			return gormTx.WithContext(ctx)
		}
	}

	return fallback.WithContext(ctx)
}
