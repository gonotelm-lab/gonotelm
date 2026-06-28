package source

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	domain "github.com/gonotelm-lab/gonotelm/internal/domain/source"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type GetSourceHandler struct {
	sourceRepo  domain.Repository
	storageRepo domain.StorageRepository
}

func NewGetSourceHandler(
	sourceRepo domain.Repository,
	storageRepo domain.StorageRepository,
) *GetSourceHandler {
	return &GetSourceHandler{
		sourceRepo:  sourceRepo,
		storageRepo: storageRepo,
	}
}

type GetSourceHandleResult struct {
	Source           *domain.Source
	FileContentUrl   string
	ParsedContentUrl string
}

func (h *GetSourceHandler) Handle(ctx context.Context, sourceId valobj.Id) (*GetSourceHandleResult, error) {
	targetSource, err := h.sourceRepo.FindById(ctx, sourceId)
	if err != nil {
		return nil, errors.WithMessagef(err, "find source failed, source_id=%s", sourceId)
	}

	if !targetSource.Status.IsReady() {
		return nil, errors.ErrParams.Msgf("source is not ready, status=%s", targetSource.Status)
	}

	result := &GetSourceHandleResult{
		Source: targetSource,
	}

	parsedPresignResult, err := h.storageRepo.PresignGet(ctx, targetSource.ParsedContentKey)
	if err != nil {
		slog.ErrorContext(ctx, "presign get object failed",
			slog.Any("err", err),
			slog.String("source_id", sourceId.String()),
			slog.String("store_key", targetSource.ParsedContentKey),
		)
	} else {
		result.ParsedContentUrl = parsedPresignResult.Url
	}

	if targetSource.Kind.IsFile() {
		fileContent, err := targetSource.GetFileContent()
		if err != nil {
			return nil, errors.WithMessagef(err, "get file content failed, source_id=%s", sourceId)
		}

		presignResult, err := h.storageRepo.PresignGet(ctx, fileContent.StoreKey)
		if err != nil {
			slog.ErrorContext(ctx, "presign get object failed",
				slog.Any("err", err),
				slog.String("source_id", sourceId.String()),
				slog.String("store_key", fileContent.StoreKey),
			)
		} else {
			result.FileContentUrl = presignResult.Url
		}
	}

	return result, nil
}
