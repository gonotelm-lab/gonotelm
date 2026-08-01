package adapter

import (
	"context"
)

// DistributedLock 通用分布式锁
type DistributedLock interface {
	// Lock 获取锁；获取失败（被占用/超时）返回错误
	Lock(ctx context.Context, key string) error
	// Unlock 释放锁
	Unlock(ctx context.Context, key string) error
	// Check 检查锁是否被持有
	Check(ctx context.Context, key string) (bool, error)
}
