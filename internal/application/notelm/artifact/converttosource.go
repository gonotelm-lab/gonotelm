package artifact

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcevo "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

type ConvertNoteToSourceCommand struct {
	ArtifactId valobj.Id
}

type ConvertNoteToSourceResult struct {
	SourceId valobj.Id
}

type ConvertNoteToSourceHandler struct {
	*baseHandler
	notebookRepo notebookrepo.Repository
	sourceRepo   sourcerepo.Repository
	storageRepo  sourcerepo.StorageRepository
	eventBus     eventbus.Publisher
}

func NewConvertNoteToSourceHandler(
	artifactRepo artifactrepo.Repository,
	sourceRepo sourcerepo.Repository,
	notebookRepo notebookrepo.Repository,
	storageRepo sourcerepo.StorageRepository,
	eventBus eventbus.Publisher,
) *ConvertNoteToSourceHandler {
	return &ConvertNoteToSourceHandler{
		baseHandler:  newBaseHandler(artifactRepo),
		notebookRepo: notebookRepo,
		sourceRepo:   sourceRepo,
		storageRepo:  storageRepo,
		eventBus:     eventBus,
	}
}

func (h *ConvertNoteToSourceHandler) Handle(
	ctx context.Context,
	cmd *ConvertNoteToSourceCommand,
) (*ConvertNoteToSourceResult, error) {
	userId := pkgcontext.GetUserId(ctx)

	artifact, err := h.handle(ctx, cmd.ArtifactId)
	if err != nil {
		return nil, err
	}

	if artifact.Kind != artifactentity.KindNote {
		return nil, errors.ErrParams.Msgf("only note artifact can be converted to source, kind=%s", artifact.Kind)
	}

	if !artifact.Status.Completed() {
		return nil, errors.ErrParams.Msgf("cannot convert non-completed artifact to source, status=%s", artifact.Status)
	}

	textContent := strings.TrimSpace(string(artifact.Result))
	if textContent == "" {
		return nil, errors.ErrParams.Msg("note artifact has no content")
	}

	nb, err := h.notebookRepo.FindById(ctx, artifact.NotebookId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get notebook failed, notebook_id=%s", artifact.NotebookId)
	}
	if nb.OwnerId != userId {
		return nil, errors.ErrPermission.Msgf("notebook access denied, notebook_id=%s", artifact.NotebookId)
	}

	if err := nb.AllowedToCreateSource(); err != nil {
		return nil, errors.WithMessage(err, "notebook not allowed to create source")
	}

	newSource, err := sourceentity.NewSource(
		artifact.NotebookId,
		sourcevo.SourceKindFile,
		userId,
		&sourceentity.ContentUnion{Kind: sourcevo.SourceKindFile},
	)
	if err != nil {
		return nil, errors.WithMessage(err, "create source failed")
	}

	title := strings.TrimSpace(artifact.Title)
	filename := noteFilename(title)
	fileSize := int64(len(artifact.Result))
	md5Sum := md5.Sum(artifact.Result)
	md5Hex := hex.EncodeToString(md5Sum[:])

	if err := newSource.UploadFile(ctx, &sourceentity.UploadFileParams{
		Filename: filename,
		MimeType: sourceentity.MimeTypeText,
		Size:     fileSize,
		Md5:      md5Hex,
	}); err != nil {
		return nil, errors.WithMessagef(err, "set file content failed, source_id=%s", newSource.Id)
	}
	newSource.UpdateTitle(title)
	newSource.MarkUploading()

	if err := h.sourceRepo.Save(ctx, newSource); err != nil {
		return nil, errors.WithMessagef(err, "save source failed, source_id=%s", newSource.Id)
	}

	fileContent, err := newSource.GetFileContent()
	if err != nil {
		return nil, errors.WithMessagef(err, "get file content failed, source_id=%s", newSource.Id)
	}

	// 将笔记文本作为 .txt 文件上传到对象存储
	if err := h.storageRepo.UploadObject(
		ctx,
		fileContent.StoreKey,
		artifact.Result,
		sourceentity.MimeTypeText,
	); err != nil {
		return nil, errors.WithMessagef(err,
			"upload note content failed, source_id=%s, store_key=%s",
			newSource.Id, fileContent.StoreKey)
	}

	// 上传成功后进入 preparing，触发下游解析
	newSource.MarkPreparing()
	if err := h.sourceRepo.Save(ctx, newSource); err != nil {
		// 回滚：删除已上传的对象，避免遗留孤儿文件
		if rollbackErr := h.storageRepo.DeleteObject(ctx, fileContent.StoreKey); rollbackErr != nil {
			slog.ErrorContext(ctx, "rollback delete uploaded object failed",
				slog.String("source_id", newSource.Id.String()),
				slog.String("store_key", fileContent.StoreKey),
				slog.Any("err", rollbackErr),
			)
		}

		return nil, errors.WithMessagef(err, "save source failed, source_id=%s", newSource.Id)
	}

	for _, evt := range newSource.PullEvents() {
		if err := h.eventBus.Publish(ctx, evt); err != nil {
			slog.ErrorContext(ctx, "publish source event failed",
				slog.String("source_id", newSource.Id.String()),
				slog.Any("err", err),
			)
		}
	}

	return &ConvertNoteToSourceResult{SourceId: newSource.Id}, nil
}

// noteFilename 将笔记标题转换为安全的 .txt 文件名
func noteFilename(title string) string {
	name := strings.TrimSpace(title)
	if name == "" {
		name = "note"
	}
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '\x00':
			return '_'
		}
		return r
	}, name)
	name = pkgstring.TruncateRune(name, 64)

	return name + ".txt"
}
