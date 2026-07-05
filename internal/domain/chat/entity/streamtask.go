package entity

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

type StreamTaskStatus string

const (
	StreamTaskStatusRunning  StreamTaskStatus = "running"
	StreamTaskStatusFinished StreamTaskStatus = "finished"
	StreamTaskStatusAborted  StreamTaskStatus = "aborted"
)

func (s StreamTaskStatus) String() string { return string(s) }

func (s StreamTaskStatus) IsRunning() bool {
	return s == StreamTaskStatusRunning
}

func (s StreamTaskStatus) IsFinished() bool {
	return s == StreamTaskStatusFinished
}

func (s StreamTaskStatus) IsAborted() bool {
	return s == StreamTaskStatusAborted
}

type StreamTask struct {
	Id             valobj.Id
	Status         StreamTaskStatus
	CreateTime     int64
	ChatId         valobj.Id
	UserId         string
	ExpireDuration time.Duration
}

func NewStreamTask(chatId valobj.Id, userId string) *StreamTask {
	return &StreamTask{
		Id:             valobj.NewUnOrderedId(),
		Status:         StreamTaskStatusRunning,
		CreateTime:     time.Now().UnixMilli(),
		ChatId:         chatId,
		UserId:         userId,
		ExpireDuration: 5 * time.Minute, // TODO make this configurable
	}
}

// Stream Event  Definition

type EventAction string

const (
	EventActionInit   EventAction = "INIT"
	EventActionAppend EventAction = "APPEND"
	EventActionSet    EventAction = "SET"
	EventActionNew    EventAction = "NEW"
)

type EventTargetPath string

const (
	EventTargetPathMessage                     EventTargetPath = "message"
	EventTargetPathCitations                   EventTargetPath = "message.citations"
	EventTargetPathFragmentThink               EventTargetPath = "message.fragments.think"
	EventTargetPathFragmentThinkContent        EventTargetPath = "message.fragments.think.content"
	EventTargetPathFragmentThinkStatus         EventTargetPath = "message.fragments.think.status"
	EventTargetPathFragmentResponse            EventTargetPath = "message.fragments.response"
	EventTargetPathFragmentResponseContentText EventTargetPath = "message.fragments.response.content.text"
	EventTargetPathFragmentResponseStatus      EventTargetPath = "message.fragments.response.status"
	EventTargetPathFragmentPhase               EventTargetPath = "message.fragments.phase"
)

// 消息任务事件
type StreamTaskEvent struct {
	Id         string    `json:"id"          msgpack:"id"`      // event id
	TaskId     valobj.Id `json:"task_id"     msgpack:"task_id"` // task id
	CreateTime int64     `json:"create_time" msgpack:"create_time"`

	Action    EventAction     `json:"action"              msgpack:"action"`
	Message   *Message        `json:"message,omitempty"   msgpack:"message,omitempty"`
	Citations []valobj.Id     `json:"citations,omitempty" msgpack:"citations,omitempty"`
	Path      EventTargetPath `json:"path"                msgpack:"path"`

	// fragment index for target fragment path. can be negative indexed
	// For example, -1 for the last fragment.
	// Only valid for fragment paths with actions which are not INIT
	Index int `json:"index,omitempty" msgpack:"index,omitempty"`

	Think    *EventThink    `json:"think,omitempty"    msgpack:"think,omitempty"`
	Response *EventResponse `json:"response,omitempty" msgpack:"response,omitempty"`
	Phase    *EventPhase    `json:"phase,omitempty"    msgpack:"phase,omitempty"`

	// Error is set when there is an error during the stream task
	Error *EventError `json:"error,omitempty" msgpack:"error,omitempty"`
}

type EventThink struct {
	Status  FragmentStatus `json:"status,omitempty" msgpack:"status,omitempty"`
	Content string         `json:"content"          msgpack:"content,omitempty"`
}

type EventResponse struct {
	Status  FragmentStatus        `json:"status,omitempty" msgpack:"status,omitempty"`
	Content *FragmentContentUnion `json:"content"          msgpack:"content,omitempty"`
}

type EventPhase struct {
	Phase *FragmentPhase `json:"phase,omitempty" msgpack:"phase,omitempty"`
}

type EventError struct {
	Message string `json:"message,omitempty" msgpack:"message,omitempty"`
}
