package schema

import (
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
)

type MessageCitation struct {
	DocId    string `json:"doc_id"`
	SourceId string `json:"source_id"`
}

type Message struct {
	Id         string                        `json:"id"`
	CreateTime int64                         `json:"create_time"`
	UpdateTime int64                         `json:"update_time"`
	ChatId     string                        `json:"chat_id"`
	UserId     string                        `json:"user_id"`
	Role       string                        `json:"role"`
	Fragments  []*chatentity.MessageFragment `json:"fragments,omitempty"`
	SeqNo      int64                         `json:"seq_no"`
	Citations  []MessageCitation             `json:"citations,omitempty"`
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
		Citations:  toMessageCitations(msg.Citations),
	}
}

func ToMessages(messages []*chatentity.Message) []*Message {
	resp := make([]*Message, 0, len(messages))
	for _, msg := range messages {
		resp = append(resp, ToMessage(msg))
	}

	return resp
}

func toMessageCitations(citations []chatentity.MessageCitation) []MessageCitation {
	if len(citations) == 0 {
		return nil
	}

	items := make([]MessageCitation, 0, len(citations))
	for _, citation := range citations {
		items = append(items, MessageCitation{
			DocId:    citation.DocId.String(),
			SourceId: citation.SourceId.String(),
		})
	}

	return items
}
