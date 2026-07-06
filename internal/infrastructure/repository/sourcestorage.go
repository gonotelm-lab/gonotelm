package repository

import (
	"context"

	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type SourceStorageRepositoryImpl struct {
	storage storage.Storage
}

func NewSourceStorageRepository(storage storage.Storage) sourcerepo.StorageRepository {
	return &SourceStorageRepositoryImpl{
		storage: storage,
	}
}

func (s *SourceStorageRepositoryImpl) PresignUpload(ctx context.Context, fileContent *sourceentity.FileSourceContent) (*sourcerepo.PresignUploadResult, error) {
	presignResult, err := s.storage.PresignedPostPolicy(ctx,
		&storage.PresignedPostPolicyRequest{
			Key:           fileContent.StoreKey,
			ContentType:   fileContent.Format,
			ContentLength: fileContent.Size,
			Filename:      fileContent.Filename,
			Md5:           fileContent.Md5,
		})
	if err != nil {
		return nil, err
	}

	return &sourcerepo.PresignUploadResult{
		Method:  presignResult.Method,
		Url:     presignResult.Url,
		Forms:   presignResult.Forms,
		Headers: presignResult.Headers,
	}, nil
}

func (s *SourceStorageRepositoryImpl) PresignGet(ctx context.Context, storeKey string) (*sourcerepo.PresignGetResult, error) {
	presignResult, err := s.storage.PresignedGetObject(ctx,
		&storage.PresignedGetObjectRequest{
			Key: storeKey,
		})
	if err != nil {
		return nil, err
	}

	return &sourcerepo.PresignGetResult{Url: presignResult.Url}, nil
}

func (s *SourceStorageRepositoryImpl) CheckExist(ctx context.Context, storeKey string) (bool, error) {
	_, err := s.storage.StatObject(ctx,
		&storage.StatObjectRequest{
			Key: storeKey,
		})
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (s *SourceStorageRepositoryImpl) GetObject(ctx context.Context, storeKey string) ([]byte, int64, error) {
	object, err := s.storage.GetObject(ctx, &storage.GetObjectRequest{
		Key: storeKey,
	})
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil, 0, sourcerepo.ErrObjectNotFound
		}

		return nil, 0, err
	}

	return object.Body, object.Info.Size, nil
}

func (s *SourceStorageRepositoryImpl) DeleteObject(ctx context.Context, storeKey string) error {
	return s.storage.DeleteObject(ctx, &storage.DeleteObjectRequest{
		Key: storeKey,
	})
}

func (s *SourceStorageRepositoryImpl) UploadObject(
	ctx context.Context,
	storeKey string,
	content []byte,
	contentType string,
) error {
	return s.storage.UploadObject(ctx,
		&storage.UploadObjectRequest{
			Key:         storeKey,
			Body:        content,
			ContentType: contentType,
		})
}
