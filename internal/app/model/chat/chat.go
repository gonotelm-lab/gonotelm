package chat

import "github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"

type Chat struct {
	Id         Id
	NotebookId Id
	OwnerId    string
	UpdatedAt  int64
}

func NewChat(
	sc *schema.Chat,
) *Chat {
	return &Chat{
		Id:         Id(sc.Id),
		NotebookId: Id(sc.NotebookId),
		OwnerId:    sc.OwnerId,
		UpdatedAt:  sc.UpdatedAt,
	}
}
