package entity

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
)

type Access struct {
	Notebook *Notebook
	Role     vo.Role
}

// 检查userId是否可以添加为笔记本的成员
func (a *Access) CanAcceptAsMember(userId valobj.Uid) bool {
	return a != nil && a.Notebook != nil && !userId.IsZero() && userId != a.Notebook.OwnerId
}

// 是否可读
func (a *Access) CanView() bool { return a.Role.CanView() }

// 是否可以管理内容 比如增删改sources和artifacts
func (a *Access) CanManageContent() bool { return a.Role.CanManageContent() }

// 是否管理笔记本 比如改标题 删除笔记本
func (a *Access) CanManageNotebook() bool { return a.Role.CanManageNotebook() }

// 是否可以管理笔记本的分享
func (a *Access) CanManageSharing() bool { return a.Role.CanManageSharing() }

func (a *Access) CanViewReturnErr() error {
	if a.CanView() {
		return nil
	}

	return errors.ErrNotebookNotFound.Msgf("notebook %s not found", a.Notebook.Id)
}
