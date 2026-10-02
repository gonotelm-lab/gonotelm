package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type Service struct {
	objectStore adapter.ObjectStore
}

func New(objectStore adapter.ObjectStore) Service {
	return Service{
		objectStore: objectStore,
	}
}

func (s *Service) GetSourceDetail(
	ctx context.Context,
	source *entity.Source,
) (*entity.SourceDetail, error) {
	sourceDetail := &entity.SourceDetail{
		Source: source,
		Access: &entity.SourceAccess{},
	}

	if source.ParsedContentKey.Valid() {
		url, err := s.objectStore.PresignGet(ctx, source.ParsedContentKey)
		if err != nil {
			return nil, errors.WithMessagef(err, "presign get parsed content failed, source_id=%s", source.Id)
		}
		sourceDetail.Access.ParsedContentUrl = url
	}

	if source.Kind.IsFile() {
		fc, err := source.GetFileContent()
		if err != nil {
			return nil, errors.WithMessagef(err, "get file content failed, source_id=%s", source.Id)
		}

		url, err := s.objectStore.PresignGet(ctx, fc.StoreKey)
		if err != nil {
			return nil, errors.WithMessagef(err, "presign get file content failed, source_id=%s", source.Id)
		}
		sourceDetail.Access.FileContentUrl = url
	}

	return sourceDetail, nil
}

func (s *Service) ListSourcesDetail(
	ctx context.Context,
	sources []*entity.Source,
) ([]*entity.SourceDetail, error) {
	const (
		batchSize      = 5
		maxConcurrency = 10
	)

	// 填充解析后的内容
	details, err := batch.ParallelMap(
		ctx,
		sources,
		batchSize,
		maxConcurrency,
		func(ctx context.Context, batch []*entity.Source) ([]*entity.SourceDetail, error) {
			sourceDetails := make([]*entity.SourceDetail, 0, len(batch))
			for _, source := range batch {
				sourceDetail, err := s.GetSourceDetail(ctx, source)
				if err != nil {
					return nil, errors.WithMessagef(err, "get source detail failed, source_id=%s", source.Id)
				}

				sourceDetails = append(sourceDetails, sourceDetail)
			}

			return sourceDetails, nil
		},
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "get source details failed")
	}

	return details, nil
}
