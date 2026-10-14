package database

import "context"

type txContextKey struct{}

// WithTx 把事务句柄放进 ctx，store 实现据此自动并入事务
// 句柄类型由具体实现决定（postgres 下是 *gorm.DB），本包只透传，不依赖具体实现
func WithTx(ctx context.Context, tx any) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

func TxFromContext(ctx context.Context) (any, bool) {
	tx := ctx.Value(txContextKey{})

	return tx, tx != nil
}

// Transactor 通用事务入口：fn 中所有走 ctx 的 store 调用在同一事务里提交或回滚
type Transactor interface {
	Transaction(ctx context.Context, fn func(ctx context.Context) error) error
}
