package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// ConfirmCoverUploadHandler 把上传完成的在途 key 转为生效封面。
type ConfirmCoverUploadHandler struct {
	*baseHandler
	distLock adapter.DistributedLock
}

func NewConfirmCoverUploadHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	userRepo identrepo.UserRepository,
	objectStore adapter.ObjectStore,
	distLock adapter.DistributedLock,
) *ConfirmCoverUploadHandler {
	h := &ConfirmCoverUploadHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService, userRepo, objectStore).withObjectStore(objectStore),
		distLock:    distLock,
	}

	return h
}

type ConfirmCoverUploadCommand struct {
	NotebookId valobj.Id
	StoreKey   string
}

func (cmd *ConfirmCoverUploadCommand) validate() (valobj.StoreKey, error) {
	key, err := valobj.DecodeStoreKey(cmd.StoreKey)
	if err != nil {
		return valobj.StoreKey{}, notebookerrors.ErrInvalidCover
	}
	if err := notebookentity.CheckCoverKey(key); err != nil {
		return valobj.StoreKey{}, notebookerrors.ErrInvalidCover
	}

	return key, nil
}

type ConfirmCoverUploadHandleResult struct {
	CoverUrl string
}

func (h *ConfirmCoverUploadHandler) Handle(
	ctx context.Context,
	cmd *ConfirmCoverUploadCommand,
) (*ConfirmCoverUploadHandleResult, error) {
	key, err := cmd.validate()
	if err != nil {
		return nil, err
	}

	result, err := withNotebookAccess(h.baseHandler, ctx, cmd.NotebookId, accessManageNotebook,
		func(ctx context.Context, access *notebookentity.Access) (*ConfirmCoverUploadHandleResult, error) {
			return h.confirmWithLock(ctx, access.Notebook, key)
		})
	if err != nil {
		return nil, errors.WithMessagef(err, "confirm cover upload failed, notebook_id=%s", cmd.NotebookId)
	}

	return result, nil
}

func (h *ConfirmCoverUploadHandler) confirmWithLock(
	ctx context.Context,
	n *notebookentity.Notebook,
	key valobj.StoreKey,
) (*ConfirmCoverUploadHandleResult, error) {
	var result *ConfirmCoverUploadHandleResult
	err := h.distLock.WithLock(ctx, notebookCoverLockKey(n.Id), func(ctx context.Context) error {
		var err error
		result, err = h.confirm(ctx, n, key)
		return err
	})
	if err != nil {
		return nil, errors.WithMessagef(err, "with cover lock failed, notebook_id=%s", n.Id)
	}

	return result, nil
}

func (h *ConfirmCoverUploadHandler) confirm(
	ctx context.Context,
	n *notebookentity.Notebook,
	key valobj.StoreKey,
) (*ConfirmCoverUploadHandleResult, error) {
	// 只认可本次 presign 下发且尚未消费的 key
	if !n.PendingCover.Valid() || n.PendingCover != key {
		return nil, notebookerrors.ErrInvalidCover
	}

	exists, err := h.objectStore.CheckExist(ctx, key)
	if err != nil {
		return nil, errors.WithMessagef(err, "check cover object failed, notebook_id=%s", n.Id)
	}
	if !exists {
		// presign 后从未传完，检查点还没来得及删
		return nil, notebookerrors.ErrInvalidCover
	}

	head, info, err := h.objectStore.GetPartialObject(ctx, key, 0, notebookentity.CoverHeadProbeBytes)
	if err != nil {
		return nil, errors.WithMessagef(err, "get cover object head failed, notebook_id=%s", n.Id)
	}
	// 检查客户端上传的是不是真的是图片
	if _, err := notebookentity.ValidateCover(info.Size, head); err != nil {
		h.deleteCoverObject(ctx, n.Id, key)
		return nil, err
	}

	oldCover := n.Cover
	if err := n.SetCover(key); err != nil {
		return nil, errors.WithMessagef(err, "set notebook cover failed, notebook_id=%s", n.Id)
	}
	if err := h.notebookRepo.Save(ctx, n); err != nil {
		return nil, errors.WithMessage(err, "save notebook cover failed")
	}

	// 旧封面删掉
	h.deleteCoverObject(ctx, n.Id, oldCover)

	return &ConfirmCoverUploadHandleResult{
		CoverUrl: h.storeKeyResolver.ResolvePublic(ctx, key),
	}, nil
}
