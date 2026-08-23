package adapter

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
)

type StorageAdapterImpl struct {
	store storage.Storage
}

func NewStorageAdapter(store storage.Storage) adapter.StorageAdapter {
	return &StorageAdapterImpl{store: store}
}

var _ adapter.StorageAdapter = &StorageAdapterImpl{}

func (s *StorageAdapterImpl) DeleteObject(ctx context.Context, key string) error {
	return s.store.DeleteObject(ctx, &storage.DeleteObjectRequest{Key: key})
}

func (s *StorageAdapterImpl) PresignGet(ctx context.Context, key string) (string, error) {
	resp, err := s.store.PresignedGetObject(ctx, &storage.PresignedGetObjectRequest{Key: key})
	if err != nil {
		return "", err
	}
	return resp.Url, nil
}
