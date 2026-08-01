package adapter

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
)

type StorageGatewayImpl struct {
	store storage.Storage
}

func NewStorageGateway(store storage.Storage) adapter.StorageGateway {
	return &StorageGatewayImpl{store: store}
}

var _ adapter.StorageGateway = &StorageGatewayImpl{}

func (s *StorageGatewayImpl) DeleteObject(ctx context.Context, key string) error {
	return s.store.DeleteObject(ctx, &storage.DeleteObjectRequest{Key: key})
}

func (s *StorageGatewayImpl) PresignGet(ctx context.Context, key string) (string, error) {
	resp, err := s.store.PresignedGetObject(ctx, &storage.PresignedGetObjectRequest{Key: key})
	if err != nil {
		return "", err
	}
	return resp.Url, nil
}
