package entity

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNotebookMember(t *testing.T) {
	member, err := NewNotebookMember(valobj.NewId(), valobj.NewUid(), vo.RoleCollaborator)
	require.NoError(t, err)
	assert.Equal(t, vo.RoleCollaborator, member.Role)
}

func TestNewNotebookMemberValidation(t *testing.T) {
	_, err := NewNotebookMember(valobj.Id{}, valobj.NewUid(), vo.RoleViewer)
	assert.ErrorIs(t, err, notebookerrors.ErrInvalidNotebookId)

	_, err = NewNotebookMember(valobj.NewId(), valobj.Uid{}, vo.RoleViewer)
	assert.ErrorIs(t, err, notebookerrors.ErrInvalidUserId)

	_, err = NewNotebookMember(valobj.NewId(), valobj.NewUid(), vo.RoleOwner)
	assert.ErrorIs(t, err, notebookerrors.ErrInvalidMemberRole)
}

func TestNotebookMemberChangeRole(t *testing.T) {
	member, err := NewNotebookMember(valobj.NewId(), valobj.NewUid(), vo.RoleViewer)
	require.NoError(t, err)

	require.NoError(t, member.ChangeRole(vo.RoleCollaborator))
	assert.Equal(t, vo.RoleCollaborator, member.Role)

	err = member.ChangeRole(vo.RoleOwner)
	assert.ErrorIs(t, err, notebookerrors.ErrInvalidMemberRole)
	assert.Equal(t, vo.RoleCollaborator, member.Role)
}

func TestNotebookMemberDelete(t *testing.T) {
	notebookId := valobj.NewId()
	userId := valobj.NewUid()
	member, err := NewNotebookMember(notebookId, userId, vo.RoleViewer)
	require.NoError(t, err)

	member.Delete()
	assert.True(t, member.IsDeleted())

	events := member.PullEvents()
	require.Len(t, events, 1)
	evt, ok := events[0].(*notebookevent.MemberEvent)
	require.True(t, ok)
	assert.Equal(t, notebookevent.EventActionMemberDeleted, evt.Action())
	assert.Equal(t, notebookId, evt.NotebookId())
	assert.True(t, evt.UserId().EqualsTo(userId))
}
