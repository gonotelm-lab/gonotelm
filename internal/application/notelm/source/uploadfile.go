package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type PresignUploadFileHandler struct {
	*baseHandler
	objectStore adapter.ObjectStore
	keyFactory  adapter.StoreKeyFactory
}

func NewPresignUploadFileHandler(
	sourceRepo sourcerepo.Repository,
	objectStore adapter.ObjectStore,
	keyFactory adapter.StoreKeyFactory,
) *PresignUploadFileHandler {
	return &PresignUploadFileHandler{
		baseHandler: newBaseHandler(sourceRepo),
		objectStore: objectStore,
		keyFactory:  keyFactory,
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
) (*adapter.PresignUploadResult, error) {
	targetSource, err := h.handle(ctx, cmd.SourceId)
	if err != nil {
		return nil, err
	}

	err = targetSource.UploadFile(ctx, &sourceentity.UploadFileParams{
		Filename: cmd.Filename,
		MimeType: cmd.MimeType,
		Size:     cmd.Size,
		Md5:      cmd.Md5,
	}, h.keyFactory)
	if err != nil {
		return nil, errors.WithMessagef(err, "upload file failed, source_id=%s", cmd.SourceId)
	}

	fileContent, err := targetSource.GetFileContent()
	if err != nil {
		return nil, errors.WithMessagef(err, "get file content failed, source_id=%s", cmd.SourceId)
	}

	// get presign url for uploading the target file
	presignResult, err := h.objectStore.PresignUpload(ctx, fileContent.StoreKey, &adapter.UploadOptions{
		ContentType:   fileContent.Format,
		ContentLength: fileContent.Size,
		Filename:      fileContent.Filename,
		Md5:           fileContent.Md5,
	})
	if err != nil {
		return nil, errors.WithMessagef(err, "presign upload object failed, source_id=%s", cmd.SourceId)
	}

	targetSource.UpdateTitle(cmd.Filename)
	targetSource.MarkUploading()
	err = h.sourceRepo.Save(ctx, targetSource)
	if err != nil {
		return nil, errors.WithMessagef(err, "save source failed, source_id=%s", cmd.SourceId)
	}

	return presignResult, nil
}
