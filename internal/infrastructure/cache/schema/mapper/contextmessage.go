package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/bytedance/sonic"
)

func ContextMessageToSchema(msg *entity.ContextMessage) (*schema.ChatContextMessage, error) {
	raw, err := sonic.Marshal(msg.Message)
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	return &schema.ChatContextMessage{
		Id:        msg.Id,
		CreatedAt: msg.CreateTime,
		Message:   raw,
	}, nil
}

func ContextMessagesToSchema(messages []*entity.ContextMessage) ([]*schema.ChatContextMessage, error) {
	schemas := make([]*schema.ChatContextMessage, 0, len(messages))
	for idx, msg := range messages {
		sch, err := ContextMessageToSchema(msg)
		if err != nil {
			return nil, errors.WithMessagef(err, "convert context message at idx=%d failed", idx)
		}
		schemas = append(schemas, sch)
	}

	return schemas, nil
}

func ContextMessageFromSchema(sch *schema.ChatContextMessage) (*entity.ContextMessage, error) {
	einoMsg, err := sch.ToEino()
	if err != nil {
		return nil, err
	}

	return &entity.ContextMessage{
		Id:         sch.Id,
		CreateTime: sch.CreatedAt,
		Message:    einoMsg,
	}, nil
}

func ContextMessagesFromSchema(schemas []*schema.ChatContextMessage) ([]*entity.ContextMessage, error) {
	messages := make([]*entity.ContextMessage, 0, len(schemas))
	for idx, sch := range schemas {
		msg, err := ContextMessageFromSchema(sch)
		if err != nil {
			return nil, errors.WithMessagef(err, "convert context message at idx=%d failed", idx)
		}
		messages = append(messages, msg)
	}

	return messages, nil
}
