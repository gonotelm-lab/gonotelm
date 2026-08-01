package event

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

const (
	StreamTaskTopic = "inner.gonotelm.chat.streamtask"
)

type StreamTaskEventAction string

const (
	// 事件流正常结束
	StreamTaskEventActionFinish StreamTaskEventAction = "streamtask.finished"
	// 事件流被终止
	StreamTaskEventActionAbort StreamTaskEventAction = "streamtask.aborted"
)

type StreamTaskEvent struct {
	event.BaseInnerEvent

	action    StreamTaskEventAction
	taskId    valobj.Id
	chatId    valobj.Id
	sourceIds []valobj.Id
}

func NewStreamTaskFinishEvent(chatId, taskId valobj.Id, sourceIds []valobj.Id) *StreamTaskEvent {
	return &StreamTaskEvent{
		chatId:    chatId,
		taskId:    taskId,
		action:    StreamTaskEventActionFinish,
		sourceIds: sourceIds,
	}
}

func NewStreamTaskAbortEvent(chatId, taskId valobj.Id, sourceIds []valobj.Id) *StreamTaskEvent {
	return &StreamTaskEvent{
		chatId:    chatId,
		taskId:    taskId,
		action:    StreamTaskEventActionAbort,
		sourceIds: sourceIds,
	}
}

func (e *StreamTaskEvent) TaskId() valobj.Id {
	return e.taskId
}

func (e *StreamTaskEvent) ChatId() valobj.Id {
	return e.chatId
}

func (e *StreamTaskEvent) SourceIds() []valobj.Id {
	return e.sourceIds
}

func (e *StreamTaskEvent) Topic() string {
	return StreamTaskTopic
}

func (e *StreamTaskEvent) Action() StreamTaskEventAction {
	return e.action
}

func (e *StreamTaskEvent) Key() string {
	return e.taskId.String()
}
