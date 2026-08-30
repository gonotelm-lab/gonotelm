package adapter

import (
	"context"
)

type StorageAdapter interface {
	DeleteObject(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string) (string, error)
}
