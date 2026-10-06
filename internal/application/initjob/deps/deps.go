// Package deps carries what the initialization tasks need. It lives in its own
// package so that both the task implementations and the registry can import it
// without an import cycle.
package deps

import (
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"

	redisv9 "github.com/redis/go-redis/v9"
)

// Instance is wired once in internal/bootstrap/initjob.go. Add a field here when a
// task needs more infrastructure.
type Instance struct {
	Database    *database.Dao
	Redis       redisv9.UniversalClient
	ObjectStore adapter.ObjectStore
	KeyFactory  adapter.StoreKeyFactory
	Cfg         *conf.InitJobConfig
}
