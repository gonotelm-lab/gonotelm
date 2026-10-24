package adapter

import (
	"context"
	"time"
)

// RetrySpec 加锁重试策略。
type RetrySpec struct {
	// Tries 加锁尝试次数；<=0 时按 1 次处理
	Tries int
	// Interval 两次尝试之间的等待间隔；Tries<=1 时忽略
	Interval time.Duration
}

// DefaultRetrySpec 默认加锁重试：共 2 次，间隔 1s。
var DefaultRetrySpec = RetrySpec{
	Tries:    2,
	Interval: time.Second,
}

// DistributedLock 通用分布式锁
type DistributedLock interface {
	// Lock 获取锁；获取失败（被占用/超时）返回错误
	Lock(ctx context.Context, key string) error
	// Unlock 释放锁
	Unlock(ctx context.Context, key string) error
	// Check 检查锁是否被持有
	Check(ctx context.Context, key string) (bool, error)
	// WithLock 持锁执行 fn，结束后自动 Unlock（不重试）
	WithLock(ctx context.Context, key string, fn func(ctx context.Context) error) error
	// WithLockWithRetry 持锁执行 fn；仅对加锁失败（ErrLockNotAcquired）按 spec 重试，耗尽仍失败则返回 err
	WithLockWithRetry(ctx context.Context, key string, spec RetrySpec, fn func(ctx context.Context) error) error
}
