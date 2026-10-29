package entity

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	pinerrors "github.com/gonotelm-lab/gonotelm/internal/domain/pin/errors"
)

const MaxPinsAllowed = 50

type UserNotebookPin struct {
	UserId     valobj.Uid
	NotebookId valobj.Id
	PinnedAt   valobj.Time
}

func NewUserNotebookPin(userId valobj.Uid, notebookId valobj.Id) (*UserNotebookPin, error) {
	p := &UserNotebookPin{
		UserId:     userId,
		NotebookId: notebookId,
		PinnedAt:   valobj.NewTime(),
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *UserNotebookPin) validate() error {
	if p.UserId.IsZero() {
		return pinerrors.ErrInvalidUserId
	}
	if p.NotebookId.IsZero() {
		return pinerrors.ErrInvalidNotebookId
	}
	return nil
}

func (p *UserNotebookPin) TouchPinnedAt() {
	p.PinnedAt = valobj.NewTime()
}
