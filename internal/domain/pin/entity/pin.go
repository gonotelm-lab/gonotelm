package entity

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	pinerrors "github.com/gonotelm-lab/gonotelm/internal/domain/pin/errors"
)

// MaxPinsPerScope 每个列表 scope（owned / shared）的置顶上限。
const MaxPinsPerScope = 20

// UserNotebookPin 用户对某 notebook 的个人置顶关系（非 notebook 属性）。
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

// TouchPinnedAt 再置顶时刷新时间，使本子排到置顶区最前。
func (p *UserNotebookPin) TouchPinnedAt() {
	p.PinnedAt = valobj.NewTime()
}
