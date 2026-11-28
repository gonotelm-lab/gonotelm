// Package migration 按 database.type 选择数据库引擎的 schema 迁移。
// 各引擎自己负责建库、连接与执行迁移，调用方只提供类型和连接配置。
package migration

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gonotelm-lab/gonotelm/migration/db/clickhouse"
	"github.com/gonotelm-lab/gonotelm/migration/db/postgres18"
	pkgsql "github.com/gonotelm-lab/gonotelm/pkg/sql"
)

// UpFunc 把一个数据库迁到最新 schema：库不存在时先建库，再增量应用迁移。
type UpFunc func(ctx context.Context, cfg *pkgsql.Config) error

// drivers 按 database.type 注册迁移实现。
var drivers = map[string]UpFunc{
	"clickhouse": clickhouse.Up,
	"postgres":   postgres18.Up,
}

// typeAliases 把常见写法归一化到 drivers 的键。
var typeAliases = map[string]string{
	"postgresql": "postgres",
}

// Up 按数据库类型执行迁移，类型不区分大小写。
func Up(ctx context.Context, dbType string, cfg *pkgsql.Config) error {
	fn, err := lookup(dbType)
	if err != nil {
		return err
	}
	return fn(ctx, cfg)
}

// Supported 返回已注册的数据库类型。
func Supported() []string {
	types := make([]string, 0, len(drivers))
	for t := range drivers {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

func lookup(dbType string) (UpFunc, error) {
	key := strings.ToLower(strings.TrimSpace(dbType))
	if canonical, ok := typeAliases[key]; ok {
		key = canonical
	}
	fn, ok := drivers[key]
	if !ok {
		return nil, fmt.Errorf("migrate: unsupported database type %q (supported: %s)",
			dbType, strings.Join(Supported(), ", "))
	}
	return fn, nil
}
