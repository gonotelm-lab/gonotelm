package entity

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

type Chat struct {
	entity.Base
	NotebookId valobj.Id
	OwnerId    valobj.Uid
}

func NewChat(notebookId valobj.Id, ownerId valobj.Uid) *Chat {
	return &Chat{
		Base:       entity.NewBase(),
		NotebookId: notebookId,
		OwnerId:    ownerId,
	}
}
