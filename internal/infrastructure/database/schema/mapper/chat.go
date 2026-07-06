package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatdomain "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
)

func ChatToSchema(chat *chatdomain.Chat) *schema.Chat {
	return &schema.Chat{
		Id:         chat.Id,
		NotebookId: chat.NotebookId,
		OwnerId:    chat.OwnerId,
		UpdatedAt:  chat.UpdateTime.Value(),
	}
}

func ChatFromSchema(sch *schema.Chat) *chatdomain.Chat {
	return &chatdomain.Chat{
		Base: entity.Base{
			Id:         valobj.Id(sch.Id),
			CreateTime: valobj.NewTimeFromId(sch.Id),
			UpdateTime: valobj.NewTimeFrom(sch.UpdatedAt),
		},
		NotebookId: valobj.Id(sch.NotebookId),
		OwnerId:    sch.OwnerId,
	}
}

func ChatsFromSchema(schemas []*schema.Chat) []*chatdomain.Chat {
	chats := make([]*chatdomain.Chat, 0, len(schemas))
	for _, sch := range schemas {
		chats = append(chats, ChatFromSchema(sch))
	}
	return chats
}
