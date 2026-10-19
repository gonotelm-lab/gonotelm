package entity

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
)

type Access struct {
	Notebook *Notebook
	Role     vo.Role
}

func (a *Access) CanAcceptAsMember(userId valobj.Uid) bool {
	return a != nil && a.Notebook != nil && !userId.IsZero() && userId != a.Notebook.OwnerId
}

func (a *Access) CanView() bool           { return a.Role.CanView() }
func (a *Access) CanManageContent() bool  { return a.Role.CanManageContent() }
func (a *Access) CanManageNotebook() bool { return a.Role.CanManageNotebook() }
func (a *Access) CanManageSharing() bool  { return a.Role.CanManageSharing() }
