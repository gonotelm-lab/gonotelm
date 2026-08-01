package adapter

import (
	"context"
	"sync"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/go-redsync/redsync/v4"
	redsyncgoredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredis "github.com/redis/go-redis/v9"
)

const defaultLockExpiry = 30 * time.Second

type redisDistributedLock struct {
	rs     *redsync.Redsync
	cli    goredis.UniversalClient
	expiry time.Duration

	mu    sync.Mutex
	locks map[string]*redsync.Mutex
}

func NewRedisDistributedLock(cli goredis.UniversalClient) adapter.DistributedLock {
	return &redisDistributedLock{
		rs:     redsync.New(redsyncgoredis.NewPool(cli)),
		cli:    cli,
		expiry: defaultLockExpiry,
		locks:  make(map[string]*redsync.Mutex),
	}
}

var _ adapter.DistributedLock = &redisDistributedLock{}

func (l *redisDistributedLock) Lock(ctx context.Context, key string) error {
	l.mu.Lock()
	m, ok := l.locks[key]
	if !ok {
		m = l.rs.NewMutex(key,
			redsync.WithExpiry(l.expiry),
			redsync.WithTries(3),
			redsync.WithFailFast(true),
		)
		l.locks[key] = m
	}
	l.mu.Unlock()

	if err := m.LockContext(ctx); err != nil {
		return errors.Wrapf(errors.ErrLockNotAcquired, "acquire distributed lock failed, key=%s: %s", key, err.Error())
	}

	return nil
}

func (l *redisDistributedLock) Unlock(ctx context.Context, key string) error {
	l.mu.Lock()
	m, ok := l.locks[key]
	if !ok {
		l.mu.Unlock()
		return nil
	}
	delete(l.locks, key)
	l.mu.Unlock()

	if _, err := m.UnlockContext(ctx); err != nil {
		return errors.Wrapf(errors.ErrCache, "release distributed lock failed, key=%s: %s", key, err.Error())
	}

	return nil
}

func (l *redisDistributedLock) Check(ctx context.Context, key string) (bool, error) {
	n, err := l.cli.Exists(ctx, key).Result()
	if err != nil {
		return false, errors.Wrapf(errors.ErrCache, "check distributed lock failed, key=%s: %s", key, err.Error())
	}

	return n > 0, nil
}
