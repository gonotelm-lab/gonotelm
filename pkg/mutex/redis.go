package mutex

import (
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
)

func NewRedisLock(cli redis.UniversalClient, name string) *redsync.Mutex {
	n := redsync.New(goredis.NewPool(cli))

	return n.NewMutex(name, redsync.WithTries(3), redsync.WithFailFast(true))
}
