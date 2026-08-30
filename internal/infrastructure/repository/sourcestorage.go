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
		if errors.Is(err, storage.ErrObjectNotFound) {
			return false, nil
		}

		return false, errors.WithMessage(err, "check object exist failed")
	}

	return true, nil
}

func (s *SourceStorageRepositoryImpl) GetObject(ctx context.Context, storeKey string) ([]byte, *sourcerepo.ObjectInfo, error) {
	object, err := s.storage.GetObject(ctx, &storage.GetObjectRequest{
		Key: storeKey,
	})
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil, nil, sourcerepo.ErrObjectNotFound
		}

		return nil, nil, errors.WithMessage(err, "get object failed")
	}

	return object.Body, &sourcerepo.ObjectInfo{
		Key:         object.Info.Key,
		Size:        object.Info.Size,
		ContentType: object.Info.ContentType,
	}, nil
}

func (s *SourceStorageRepositoryImpl) GetPartialObject(
	ctx context.Context,
	storeKey string,
	offset int64,
	length int64,
) ([]byte, *sourcerepo.ObjectInfo, error) {
	object, err := s.storage.GetPartialObject(ctx, &storage.GetPartialObjectRequest{
		Key:    storeKey,
		Offset: offset,
		Length: length,
	})
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil, nil, sourcerepo.ErrObjectNotFound
		}

		return nil, nil, errors.WithMessage(err, "get partial object failed")
	}

	return object.Body, &sourcerepo.ObjectInfo{
		Key:         object.Info.Key,
		Size:        object.Info.Size,
		ContentType: object.Info.ContentType,
	}, nil
}

func (s *SourceStorageRepositoryImpl) DeleteObject(ctx context.Context, storeKey string) error {
	err := s.storage.DeleteObject(ctx, &storage.DeleteObjectRequest{
		Key: storeKey,
	})
	if err != nil {
		return errors.WithMessage(err, "delete object failed")
	}

	return nil
}

func (s *SourceStorageRepositoryImpl) UploadObject(
	ctx context.Context,
	storeKey string,
	content []byte,
	contentType string,
) error {
	err := s.storage.UploadObject(ctx,
		&storage.UploadObjectRequest{
			Key:         storeKey,
			Body:        content,
			ContentType: contentType,
		})
	if err != nil {
		return errors.WithMessage(err, "upload object failed")
	}

	return nil
}
