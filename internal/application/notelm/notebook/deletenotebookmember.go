package notebook

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteNotebookMemberHandler struct {
	*baseHandler
	memberRepo notebookrepo.MemberRepository
	eventBus   eventbus.Publisher
}

type deleteNotebookMemberResult struct{}

func NewDeleteNotebookMemberHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	memberRepo notebookrepo.MemberRepository,
	userRepo identrepo.UserRepository,
	eventBus eventbus.Publisher,
) *DeleteNotebookMemberHandler {
	return &DeleteNotebookMemberHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService, userRepo, nil),
		memberRepo:  memberRepo,
		eventBus:    eventBus,
	}
}

func (h *DeleteNotebookMemberHandler) Handle(
	ctx context.Context,
	notebookId valobj.Id,
	userId valobj.Uid,
) error {
	_, err := withNotebookAccess(h.baseHandler, ctx, notebookId, accessManageSharing,
		func(ctx context.Context, access *notebookentity.Access) (*deleteNotebookMemberResult, error) {
			return h.deleteMember(ctx, access, userId)
		},
	)
	return err
}

func (h *DeleteNotebookMemberHandler) deleteMember(
	ctx context.Context,
	access *notebookentity.Access,
	userId valobj.Uid,
) (*deleteNotebookMemberResult, error) {
	notebookId := access.Notebook.Id

	member, err := h.memberRepo.FindByNotebookIdAndUserId(ctx, notebookId, userId)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, notebookerrors.ErrMemberNotFound
		}
		return nil, errors.WithMessage(err, "get notebook member failed")
	}

	member.Delete()
	if err := h.memberRepo.Save(ctx, member); err != nil {
		return nil, errors.WithMessage(err, "delete notebook member failed")
	}

	for _, evt := range member.PullEvents() {
		if err := h.eventBus.Publish(ctx, evt); err != nil {
			slog.ErrorContext(ctx, "publish notebook member deleted event failed",
				slog.String("notebook_id", notebookId.String()),
				slog.String("user_id", userId.String()),
				slog.Any("err", err),
			)
		}
	}

	return &deleteNotebookMemberResult{}, nil
}
