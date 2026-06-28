package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	domain "github.com/gonotelm-lab/gonotelm/internal/domain/source"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema/mapper"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type SourceRepositoryImpl struct {
	sourceStore dal.SourceStore
}

func NewSourceRepository(sourceStore dal.SourceStore) domain.Repository {
	return &SourceRepositoryImpl{
		sourceStore: sourceStore,
	}
}

var _ domain.Repository = &SourceRepositoryImpl{}

func (s *SourceRepositoryImpl) Save(ctx context.Context, source *domain.Source) error {
	if source.IsDeleted() {
		return s.sourceStore.DeleteById(ctx, source.Id)
	}

	sch := mapper.SourceToSchema(source)
	return s.sourceStore.Upsert(ctx, sch)
}

func (s *SourceRepositoryImpl) FindById(ctx context.Context, id valobj.Id) (*domain.Source, error) {
	source, err := s.sourceStore.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, domain.ErrSourceNotFound
		}

		return nil, err
	}

	return mapper.SourceFromSchema(source)
}

func (s *SourceRepositoryImpl) ListByNotebookId(
	ctx context.Context,
	notebookId valobj.Id,
	spec *domain.ListSpec,
) ([]*domain.Source, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}

	sources, err := s.sourceStore.ListByNotebookId(ctx, notebookId, spec.Limit, spec.Offset)
	if err != nil {
		return nil, err
	}

	return mapper.SourcesFromSchemas(sources)
}

type SourceStorageRepositoryImpl struct {
	storage storage.Storage
}

func NewSourceStorageRepository(storage storage.Storage) domain.StorageRepository {
	return &SourceStorageRepositoryImpl{
		storage: storage,
	}
}

func (s *SourceStorageRepositoryImpl) PresignUpload(ctx context.Context, fileContent *domain.FileSourceContent) (*domain.PresignUploadResult, error) {
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

	return &domain.PresignUploadResult{
		Method:  presignResult.Method,
		Url:     presignResult.Url,
		Forms:   presignResult.Forms,
		Headers: presignResult.Headers,
	}, nil
}

func (s *SourceStorageRepositoryImpl) PresignGet(ctx context.Context, storeKey string) (*domain.PresignGetResult, error) {
	presignResult, err := s.storage.PresignedGetObject(ctx,
		&storage.PresignedGetObjectRequest{
			Key: storeKey,
		})
	if err != nil {
		return nil, err
	}

	return &domain.PresignGetResult{Url: presignResult.Url}, nil
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
