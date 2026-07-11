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
	EventTargetPathMessage                     EventTargetPath = "m"           // message
	EventTargetPathCitations                   EventTargetPath = "m.citations" // message citations
	EventTargetPathFragmentThink               EventTargetPath = "m.f.tk"      // message.fragments.think
	EventTargetPathFragmentThinkContent        EventTargetPath = "m.f.tk.v"    // message.fragments.think.content
	EventTargetPathFragmentThinkStatus         EventTargetPath = "m.f.tk.st"   // message.fragments.think.status
	EventTargetPathFragmentResponse            EventTargetPath = "m.f.rsp"     // message.fragments.response
	EventTargetPathFragmentResponseContentText EventTargetPath = "m.f.rsp.v"   // message.fragments.response.content.text
	EventTargetPathFragmentResponseStatus      EventTargetPath = "m.f.rsp.st"  // message.fragments.response.status
	EventTargetPathFragmentPhase               EventTargetPath = "m.f.phase"   // message.fragments.phase
)

// 消息任务事件
type StreamTaskEvent struct {
	Id         string `json:"id" msgpack:"id"` // event id
	CreateTime int64  `json:"ct" msgpack:"ct"`

	Action    EventAction       `json:"op,omitempty"        msgpack:"op,omitempty"`
	Message   *Message          `json:"message,omitempty"   msgpack:"message,omitempty"`
	Citations []MessageCitation `json:"citations,omitempty" msgpack:"citations,omitempty"`
	Path      EventTargetPath   `json:"p,omitempty"         msgpack:"p,omitempty"`

	// fragment index for target fragment path. can be negative indexed
	// For example, -1 for the last fragment.
	// Only valid for fragment paths with actions which are not INIT
	Index int `json:"idx,omitempty" msgpack:"idx,omitempty"`

	Think    *EventThink    `json:"tk,omitempty"    msgpack:"tk,omitempty"`
	Response *EventResponse `json:"rsp,omitempty"   msgpack:"rsp,omitempty"`
	Phase    *EventPhase    `json:"phase,omitempty" msgpack:"phase,omitempty"`

	// Error is set when there is an error during the stream task
	Error *EventError `json:"error,omitempty" msgpack:"error,omitempty"`

	// 标记一个最后一个事件
	Done bool `json:"done,omitempty" msgpack:"done,omitempty"`
}

type EventThink struct {
	Status  FragmentStatus `json:"st,omitempty" msgpack:"st,omitempty"`
	Content string         `json:"v"            msgpack:"v,omitempty"`
}

type EventResponse struct {
	Status  FragmentStatus        `json:"st,omitempty" msgpack:"st,omitempty"`
	Content *FragmentContentUnion `json:"v"            msgpack:"v,omitempty"`
}

type EventPhase struct {
	Phase *FragmentPhase `json:"phase,omitempty" msgpack:"phase,omitempty"`
}

type EventError struct {
	Message string `json:"message,omitempty" msgpack:"message,omitempty"`
}
