package mapper

import (
	coreentity "github.com/gonotelm-lab/gonotelm/internal/core/entity"
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
		CreatedAt:      task.CreateTime.Value(),
		UpdatedAt:      task.UpdateTime.Value(),
		ChatId:         task.ChatId.String(),
		SourceIds:      valobj.IdsToStrings(task.SourceIds),
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

	sourceIds, err := valobj.StringsToIds(sch.SourceIds)
	if err != nil {
		return nil, err
	}

	return &entity.StreamTask{
		Base: coreentity.Base{
			Id:         id,
			CreateTime: valobj.NewTimeFrom(sch.CreatedAt),
			UpdateTime: valobj.NewTimeFrom(sch.UpdatedAt),
		},
		Status:         entity.StreamTaskStatus(sch.Status),
		ChatId:         chatId,
		SourceIds:      sourceIds,
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
