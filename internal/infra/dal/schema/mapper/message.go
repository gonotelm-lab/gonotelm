package mapper

import (
	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatdomain "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type MessageExtra struct {
	Citations []valobj.Id `json:"citations"` // source doc ids 
}

func MessageToSchema(msg *chatdomain.Message) (*schema.ChatMessage, error) {
	content, err := msg.GetFragmentBytes()
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	extra := &MessageExtra{
		Citations: msg.Citations,
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
	if err := sonic.Unmarshal(sch.Extra, &extra); err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
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
