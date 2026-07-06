package mapper

import (
	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatdomain "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type MessageExtra struct {
	Citations []chatdomain.MessageCitation `json:"citations,omitempty"`
}

func MessageToSchema(msg *chatdomain.Message) (*schema.ChatMessage, error) {
	content, err := msg.GetFragmentBytes()
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	var extra *MessageExtra
	if len(msg.Citations) > 0 {
		extra = &MessageExtra{
			Citations: msg.Citations,
		}
	}

	extraBytes, err := sonic.Marshal(extra)
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	return &schema.ChatMessage{
		Id:      msg.Id,
		ChatId:  msg.ChatId,
		UserId:  msg.UserId,
		MsgRole: int8(msg.Role),
		Content: content,
		SeqNo:   msg.SeqNo,
		Extra:   extraBytes,
	}, nil
}

func MessageFromSchema(sch *schema.ChatMessage) (*chatdomain.Message, error) {
	msg := &chatdomain.Message{
		Base: entity.Base{
			Id:         valobj.Id(sch.Id),
			CreateTime: valobj.NewTimeFromId(sch.Id),
			UpdateTime: valobj.NewTimeFromId(sch.Id),
		},
		ChatId: valobj.Id(sch.ChatId),
		UserId: sch.UserId,
		Role:   chatdomain.MessageRole(sch.MsgRole),
		SeqNo:  sch.SeqNo,
	}
	if err := msg.SetFragmentsFromBytes(sch.Content); err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	var extra MessageExtra
	if len(sch.Extra) > 0 {
		if err := sonic.Unmarshal(sch.Extra, &extra); err != nil {
			return nil, errors.Wrap(errors.ErrSerde, err.Error())
		}
	}
	msg.SetCitations(extra.Citations)

	return msg, nil
}

func MessagesFromSchema(schemas []*schema.ChatMessage) ([]*chatdomain.Message, error) {
	messages := make([]*chatdomain.Message, 0, len(schemas))
	for idx, sch := range schemas {
		msg, err := MessageFromSchema(sch)
		if err != nil {
			return nil, errors.WithMessagef(err, "convert message at idx=%d failed", idx)
		}
		messages = append(messages, msg)
	}

	return messages, nil
}
