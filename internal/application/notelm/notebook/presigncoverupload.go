package notebook

import (
	"context"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// CoverUploadPresignHandler 下发封面上传 URL；pending key 已存在时幂等复用。
type CoverUploadPresignHandler struct {
	*baseHandler
	keyFactory   adapter.StoreKeyFactory
	distLock     adapter.DistributedLock
	cleaner      *cleanup.Cleaner
	cleanupDelay time.Duration
}

func NewCoverUploadPresignHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	userRepo identrepo.UserRepository,
	objectStore adapter.ObjectStore,
	keyFactory adapter.StoreKeyFactory,
	distLock adapter.DistributedLock,
	cleaner *cleanup.Cleaner,
) *CoverUploadPresignHandler {
	h := &CoverUploadPresignHandler{
		baseHandler:  newBaseHandler(notebookRepo, notebookAccessService, userRepo, objectStore).withObjectStore(objectStore),
		keyFactory:   keyFactory,
		distLock:     distLock,
		cleaner:      cleaner,
		cleanupDelay: notebookentity.CoverCleanupDelay, // 固定 entity.CoverCleanupDelay
	}

	return h
}

type CoverUploadPresignHandleCommand struct {
	NotebookId  valobj.Id
	Size        int64
	ContentType string
}

func (m *CoverUploadPresignHandleCommand) validate() error {
	if !notebookentity.CoverContentTypeValid(m.ContentType) ||
		m.Size <= 0 ||
		m.Size > notebookentity.MaxCoverBytes {
		return notebookerrors.ErrInvalidCover
	}

	return nil
}

type CoverUploadPresignHandleResult struct {
	StoreKey string
	Presign  *adapter.PresignUploadResult
}

func (h *CoverUploadPresignHandler) Handle(
	ctx context.Context,
	cmd *CoverUploadPresignHandleCommand,
) (*CoverUploadPresignHandleResult, error) {
	result, err := withNotebookAccess(h.baseHandler, ctx, cmd.NotebookId, accessManageNotebook,
		func(ctx context.Context, access *notebookentity.Access) (*CoverUploadPresignHandleResult, error) {
			return h.presignWithLock(ctx, access.Notebook, cmd)
		})
	if err != nil {
		return nil, errors.WithMessagef(err, "presign cover upload failed, notebook_id=%s", cmd.NotebookId)
	}

	return result, nil
}

func (h *CoverUploadPresignHandler) presignWithLock(
	ctx context.Context,
	n *notebookentity.Notebook,
	cmd *CoverUploadPresignHandleCommand,
) (*CoverUploadPresignHandleResult, error) {
	var result *CoverUploadPresignHandleResult
	err := h.distLock.WithLock(ctx, notebookCoverLockKey(n.Id), func(ctx context.Context) error {
		var err error
		result, err = h.presign(ctx, n, cmd)
		return err
	})
	if err != nil {
		return nil, errors.WithMessagef(err, "with cover lock failed, notebook_id=%s", n.Id)
	}

	return result, nil
}

func (h *CoverUploadPresignHandler) presign(
	ctx context.Context,
	n *notebookentity.Notebook,
	cmd *CoverUploadPresignHandleCommand,
) (*CoverUploadPresignHandleResult, error) {
	if err := cmd.validate(); err != nil {
		return nil, err
	}

	key, isNew, err := n.BeginCoverFlow(h.keyFactory)
	if err != nil {
		return nil, errors.WithMessagef(err, "generate cover key failed, notebook_id=%s", n.Id)
	}

	// 申请上传链接
	presign, err := h.objectStore.PresignUpload(ctx, key, &adapter.UploadOptions{
		ContentType:   cmd.ContentType,
		ContentLength: cmd.Size,
	})
	if err != nil {
		return nil, errors.WithMessagef(err, "presign cover object failed, notebook_id=%s", n.Id)
	}

	// 登记在途 key；旧封面对象留给 confirm 成功后才删
	if isNew {
		if err := h.notebookRepo.Save(ctx, n); err != nil {
			return nil, errors.WithMessage(err, "save notebook pending cover failed")
		}
	}

	// 复用场景也续注一次检查点，失败不阻塞上传
	if err := h.cleaner.ScheduleCoverCleanup(ctx, n.Id, []valobj.StoreKey{key}, h.cleanupDelay); err != nil {
		slog.WarnContext(ctx, "schedule cover cleanup failed",
			slog.String("notebook_id", n.Id.String()),
			slog.Any("err", err),
		)
	} else {
		slog.InfoContext(ctx, "schedule cover clean succeeded", slog.String("notebook_id", n.Id.String()))
	}

	return &CoverUploadPresignHandleResult{StoreKey: key.Encode(), Presign: presign}, nil
}
