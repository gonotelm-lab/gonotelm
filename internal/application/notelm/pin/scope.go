package pin

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
)

type scopeResolver struct {
	accessService *notebookservice.AccessService
}

// resolveAccess 只认 owner / 显式成员；不走 Resolve（避免 public viewer 误放行）。
func (r *scopeResolver) resolveAccess(
	ctx context.Context,
	actor valobj.Uid,
	notebookId valobj.Id,
) error {
	_, err := r.accessService.ResolveOwnerOrMember(ctx, notebookId, actor)
	return err
}
