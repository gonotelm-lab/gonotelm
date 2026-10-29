package event

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

const (
	TopicNotebookMemberEvent = "gonotelm.notebook.member.event"
)

const (
	EventActionMemberDeleted EventAction = "notebook.member.deleted"
)

type MemberEvent struct {
	notebookId valobj.Id
	userId     valobj.Uid
	action     EventAction
}

func (e *MemberEvent) NotebookId() valobj.Id {
	return e.notebookId
}

func (e *MemberEvent) UserId() valobj.Uid {
	return e.userId
}

func (e *MemberEvent) Action() EventAction {
	return e.action
}

func (e *MemberEvent) Category() event.Category {
	return event.CategoryInProcess
}

func (e *MemberEvent) Topic() string {
	return TopicNotebookMemberEvent
}

func (e *MemberEvent) Key() string {
	return e.notebookId.String()
}

func (e *MemberEvent) Value() any {
	return e
}

func (e *MemberEvent) Headers() []event.Header {
	return nil
}

func NewMemberDeletedEvent(notebookId valobj.Id, userId valobj.Uid) *MemberEvent {
	return &MemberEvent{
		notebookId: notebookId,
		userId:     userId,
		action:     EventActionMemberDeleted,
	}
}
