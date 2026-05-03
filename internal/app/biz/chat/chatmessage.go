package chat

import (
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
)

type ChatMessageBiz struct {
	chatMessageStore dal.ChatMessageStore
}

func NewChatMessageBiz(chatMessageStore dal.ChatMessageStore) *ChatMessageBiz {
	return &ChatMessageBiz{chatMessageStore: chatMessageStore}
}
