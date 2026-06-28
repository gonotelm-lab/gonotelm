package source

import (
	"context"

	domain "github.com/gonotelm-lab/gonotelm/internal/domain/source"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type PresignUploadFileHandler struct {
	sourceRepo  domain.Repository
	storageRepo domain.StorageRepository
}

func NewPresignUploadFileHandler(
	sourceRepo domain.Repository,
	storageRepo domain.StorageRepository,
) *PresignUploadFileHandler {
	return &PresignUploadFileHandler{
		sourceRepo:  sourceRepo,
		storageRepo: storageRepo,
	}
}

type PresignUploadFileHandleCommand struct {
	SourceId uuid.UUID
	Filename string
	MimeType string
	Size     int64
	Md5      string
}

func (h *PresignUploadFileHandler) Handle(
	ctx context.Context,
	cmd *PresignUploadFileHandleCommand,
) (*domain.PresignUploadResult, error) {
	targetSource, err := h.sourceRepo.FindById(ctx, cmd.SourceId)
	if err != nil {
		return nil, errors.WithMessagef(err, "find source failed, source_id=%s", cmd.SourceId)
	}

	err = targetSource.UploadFile(ctx, &domain.UploadFileParams{
		Filename: cmd.Filename,
		MimeType: cmd.MimeType,
		Size:     cmd.Size,
		Md5:      cmd.Md5,
	})
	if err != nil {
		return nil, errors.WithMessagef(err, "upload file failed, source_id=%s", cmd.SourceId)
	}

	fileContent, err := targetSource.GetFileContent()
	if err != nil {
		return nil, errors.WithMessagef(err, "get file content failed, source_id=%s", cmd.SourceId)
	}

	// get presign url for uploading the target file
	presignResult, err := h.storageRepo.PresignUpload(ctx, fileContent)
	if err != nil {
		return nil, errors.WithMessagef(err, "presign upload object failed, source_id=%s", cmd.SourceId)
	}

	err = h.sourceRepo.Save(ctx, targetSource)
	if err != nil {
		return nil, errors.WithMessagef(err, "save source failed, source_id=%s", cmd.SourceId)
	}

	return presignResult, nil
}
