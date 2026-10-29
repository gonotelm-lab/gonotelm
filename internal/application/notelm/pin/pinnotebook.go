package pin

import (
	"context"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	pinentity "github.com/gonotelm-lab/gonotelm/internal/domain/pin/entity"
	pinerrors "github.com/gonotelm-lab/gonotelm/internal/domain/pin/errors"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	pinservice "github.com/gonotelm-lab/gonotelm/internal/domain/pin/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func userPinLockKey(userId valobj.Uid) string {
	return fmt.Sprintf("notelm:user:pin:lock:%s", userId)
}

type PinNotebookHandler struct {
	*scopeResolver
	pinRepo    pinrepo.Repository
	pinService *pinservice.Service
	distLock   adapter.DistributedLock
}

func NewPinNotebookHandler(
	accessService *notebookservice.AccessService,
	pinRepo pinrepo.Repository,
	pinService *pinservice.Service,
	distLock adapter.DistributedLock,
) *PinNotebookHandler {
	return &PinNotebookHandler{
		scopeResolver: &scopeResolver{accessService: accessService},
		pinRepo:       pinRepo,
		pinService:    pinService,
		distLock:      distLock,
	}
}

type PinResult struct {
	IsPinned bool
	PinnedAt int64
}

func (h *PinNotebookHandler) Handle(ctx context.Context, notebookId valobj.Id) (*PinResult, error) {
	actor := pkgcontext.GetUserId(ctx)

	if err := h.resolveAccess(ctx, actor, notebookId); err != nil {
		return nil, err
	}

	if existing, err := h.pinRepo.Find(ctx, actor, notebookId); err == nil {
		return h.refreshPin(ctx, existing) // 已经 pin 过了 则更新时间即可
	} else if !errors.Is(err, pinerrors.ErrPinNotFound) {
		return nil, errors.WithMessage(err, "find pin failed")
	}

	// 锁外预检：已满则直接拒绝，避免无谓抢锁
	if err := h.pinService.CheckPinLimit(ctx, actor); err != nil {
		return nil, err
	}

	var result *PinResult
	err := h.distLock.WithLockWithRetry(
		ctx,
		userPinLockKey(actor),
		adapter.DefaultRetrySpec,
		h.handlePin(actor, notebookId, &result),
	)
	if err != nil {
		return nil, errors.WithMessage(err, "pin notebook with lock failed")
	}

	return result, nil
}

// handlePin 返回锁内回调：再查 / 再检上限 / 落库。
func (h *PinNotebookHandler) handlePin(
	actor valobj.Uid,
	notebookId valobj.Id,
	result **PinResult,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// 锁内再查：等待锁期间可能已被 pin
		if existing, err := h.pinRepo.Find(ctx, actor, notebookId); err == nil {
			out, err := h.refreshPin(ctx, existing)
			if err != nil {
				return err
			}
			*result = out
			return nil
		} else if !errors.Is(err, pinerrors.ErrPinNotFound) {
			return errors.WithMessage(err, "find pin failed")
		}

		// 锁内再检额度，通过才能新增 pin
		if err := h.pinService.CheckPinLimit(ctx, actor); err != nil {
			return err
		}

		pin, err := pinentity.NewUserNotebookPin(actor, notebookId)
		if err != nil {
			return err
		}
		if err := h.pinRepo.Save(ctx, pin); err != nil {
			return errors.WithMessage(err, "save pin failed")
		}

		*result = &PinResult{IsPinned: true, PinnedAt: pin.PinnedAt.Value()}
		return nil
	}
}

func (h *PinNotebookHandler) refreshPin(
	ctx context.Context,
	pin *pinentity.UserNotebookPin,
) (*PinResult, error) {
	pin.TouchPinnedAt()
	if err := h.pinRepo.Save(ctx, pin); err != nil {
		return nil, errors.WithMessage(err, "refresh pin failed")
	}
	return &PinResult{IsPinned: true, PinnedAt: pin.PinnedAt.Value()}, nil
}
