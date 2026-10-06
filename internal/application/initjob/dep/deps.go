// Package deps carries what the initialization tasks need. It lives in its own
// package so that both the task implementations and the registry can import it
// without an import cycle.
package dep

import (
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"

	redisv9 "github.com/redis/go-redis/v9"
)

// Infra is wired once in internal/bootstrap/initjob.go. Add a field here when a
// task needs more infrastructure.
type Infra struct {
	Database    *database.Dao
	Redis       redisv9.UniversalClient
	ObjectStore adapter.ObjectStore
	KeyFactory  adapter.StoreKeyFactory
	Cfg         *conf.InitJobConfig
}

type Dependency struct {
	StylePreviewRepo artifactrepo.StylePreviewRepository
}
