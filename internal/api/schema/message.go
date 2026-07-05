package schema

import (
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

type Message struct {
	Id         string                        `json:"id"`
	CreateTime int64                         `json:"create_time"`
	UpdateTime int64                         `json:"update_time"`
	ChatId     string                        `json:"chat_id"`
	UserId     string                        `json:"user_id"`
	Role       string                        `json:"role"`
	Fragments  []*chatentity.MessageFragment `json:"fragments,omitempty"`
	SeqNo      int64                         `json:"seq_no"`
	Citations  []string                      `json:"citations,omitempty"`
}

func ToMessage(msg *chatentity.Message) *Message {
	if msg == nil {
		return nil
	}

	return &Message{
		Id:         msg.Id.String(),
		CreateTime: msg.CreateTime.Value(),
		UpdateTime: msg.UpdateTime.Value(),
		ChatId:     msg.ChatId.String(),
		UserId:     msg.UserId,
		Role:       msg.Role.String(),
		Fragments:  msg.Fragments,
		SeqNo:      msg.SeqNo,
		Citations:  toCitationIds(msg.Citations),
	}
}

func ToMessages(messages []*chatentity.Message) []*Message {
	resp := make([]*Message, 0, len(messages))
	for _, msg := range messages {
		resp = append(resp, ToMessage(msg))
	}

	return resp
}

func toCitationIds(citations []valobj.Id) []string {
	if len(citations) == 0 {
		return nil
	}

	ids := make([]string, 0, len(citations))
	for _, id := range citations {
		ids = append(ids, id.String())
	}

	return ids
}
