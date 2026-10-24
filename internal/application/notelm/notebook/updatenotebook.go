package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UpdateNotebookHandler struct {
	*baseHandler
}

type UpdateNotebookCommand struct {
	Name                 *string
	CustomizeDescription *bool
	Desc                 *string
}

func NewUpdateNotebookHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	userRepo identrepo.UserRepository,
) *UpdateNotebookHandler {
	return &UpdateNotebookHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService, userRepo, nil),
	}
}

func (h *UpdateNotebookHandler) Handle(
	ctx context.Context,
	id valobj.Id,
	cmd *UpdateNotebookCommand,
) error {
	_, err := withNotebookAccess(h.baseHandler, ctx, id, accessManageNotebook,
		func(ctx context.Context, access *notebookentity.Access) (*struct{}, error) {
			return nil, h.update(ctx, access, cmd)
		},
	)
	return err
}

func (h *UpdateNotebookHandler) update(
	ctx context.Context,
	access *notebookentity.Access,
	cmd *UpdateNotebookCommand,
) error {
	n := access.Notebook

	if cmd.Name != nil {
		if err := n.UpdateName(*cmd.Name); err != nil {
			return errors.WithMessage(err, "update notebook name failed")
		}
	}

	if cmd.CustomizeDescription != nil {
		if *cmd.CustomizeDescription {
			desc := ""
			if cmd.Desc != nil {
				desc = *cmd.Desc
			}
			if err := n.SetCustomizeDescription(desc); err != nil {
				return errors.WithMessage(err, "set custom description failed")
			}
		} else {
			n.ClearCustomizeDescription()
		}
	} else if cmd.Desc != nil {
		return errors.ErrParams.Msg("desc requires customize_description")
	}

	if err := h.notebookRepo.Save(ctx, n); err != nil {
		return errors.WithMessage(err, "save notebook failed")
	}

	return nil
}
