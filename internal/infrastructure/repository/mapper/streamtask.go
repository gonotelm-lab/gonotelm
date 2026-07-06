package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/vmihailenco/msgpack/v5"
)

func StreamTaskToSchema(task *entity.StreamTask) *schema.ChatMessageTask {
	return &schema.ChatMessageTask{
		Id:             task.Id.String(),
		Status:         task.Status.String(),
		CreatedAt:      task.CreateTime,
		ChatId:         task.ChatId.String(),
		UserId:         task.UserId,
		ExpireDuration: task.ExpireDuration,
	}
}

func StreamTaskFromSchema(sch *schema.ChatMessageTask) (*entity.StreamTask, error) {
	chatId, err := valobj.NewIdFromString(sch.ChatId)
	if err != nil {
		return nil, err
	}

	id, err := valobj.NewIdFromString(sch.Id)
	if err != nil {
		return nil, err
	}

	return &entity.StreamTask{
		Id:             id,
		Status:         entity.StreamTaskStatus(sch.Status),
		CreateTime:     sch.CreatedAt,
		ChatId:         chatId,
		UserId:         sch.UserId,
		ExpireDuration: sch.ExpireDuration,
	}, nil
}

func StreamTaskEventToData(event *entity.StreamTaskEvent) ([]byte, error) {
	data, err := msgpack.Marshal(event)
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}
	return data, nil
}

func StreamTaskEventFromData(data []byte) (*entity.StreamTaskEvent, error) {
	event := &entity.StreamTaskEvent{}
	if err := msgpack.Unmarshal(data, event); err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}
	return event, nil
}
