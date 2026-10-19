package entity

import (
	coreentity "github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
)

type NotebookMember struct {
	coreentity.Base

	NotebookId valobj.Id
	UserId     valobj.Uid
	Role       vo.Role
}

func NewNotebookMember(
	notebookId valobj.Id,
	userId valobj.Uid,
	role vo.Role,
) (*NotebookMember, error) {
	m := &NotebookMember{
		Base:       coreentity.NewBase(),
		NotebookId: notebookId,
		UserId:     userId,
		Role:       role,
	}

	if err := m.validate(); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *NotebookMember) validate() error {
	if m.NotebookId.IsZero() {
		return notebookerrors.ErrInvalidNotebookId
	}

	if m.UserId.IsZero() {
		return notebookerrors.ErrInvalidUserId
	}

	if !m.Role.IsMemberRole() {
		return notebookerrors.ErrInvalidMemberRole
	}

	return nil
}

func (m *NotebookMember) ChangeRole(role vo.Role) error {
	previous := m.Role
	m.Role = role
	if err := m.validate(); err != nil {
		m.Role = previous
		return err
	}

	m.UpdateTime = valobj.NewTime()
	return nil
}
