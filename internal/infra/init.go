package infra

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	cacheimpl "github.com/gonotelm-lab/gonotelm/internal/infra/cache/impl"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	dalimpl "github.com/gonotelm-lab/gonotelm/internal/infra/dal/impl"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal"
	vectordalimpl "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/impl"

	"github.com/redis/go-redis/v9"
)

var gInstances *Instances

type Instances struct {
	Dal       *dal.DAL
	VectorDal *vectordal.DAL
	Cache     *cacheimpl.Cache
	redis     redis.UniversalClient
}

func MustInit(c *conf.Config) *Instances {
	d, err := dalimpl.New(dalimpl.Type(c.Database.Type), c.SQLConfig())
	if err != nil {
		panic(err)
	}

	slog.Info("initialized dal", "type", c.Database.Type)

	vd, err := vectordalimpl.New(&c.VectorDB)
	if err != nil {
		panic(err)
	}

	slog.Info("initialized vector dal", "type", c.VectorDB.Type)

	if err := cache.Init(&c.Redis); err != nil {
		panic(err)
	}

	cacheImpl := cacheimpl.NewCache(cache.GetRedis())

	gInstances = &Instances{
		Dal:       d,
		VectorDal: vd,
		Cache:     cacheImpl,
		redis:     cache.GetRedis(),
	}

	return gInstances
}

func Close(ctx context.Context) {
	if err := gInstances.VectorDal.Close(ctx); err != nil {
		slog.ErrorContext(ctx, "close vector dal failed", slog.Any("err", err))
	}
	if err := gInstances.redis.Close(); err != nil {
		slog.ErrorContext(ctx, "close redis cache failed", slog.Any("err", err))
	}
	if err := gInstances.Dal.Close(ctx); err != nil {
		slog.ErrorContext(ctx, "close dal failed", slog.Any("err", err))
	}
}
