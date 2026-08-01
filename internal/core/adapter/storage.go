package adapter

import (
	"context"
)

type StorageGateway interface {
	DeleteObject(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string) (string, error)
}
