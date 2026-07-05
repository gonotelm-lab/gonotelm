package entity

import (
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/idgen"
)

type MessageRole int8

const (
	MessageRoleUser      MessageRole = 0
	MessageRoleAssistant MessageRole = 1
)

func (r MessageRole) String() string {
	switch r {
	case MessageRoleUser:
		return "user"
	case MessageRoleAssistant:
		return "assistant"
	default:
		return "unknown"
	}
}

type Message struct {
	entity.Base

	ChatId    valobj.Id          `json:"chat_id"`
	UserId    string             `json:"user_id"`
	Role      MessageRole        `json:"role"`
	Fragments []*MessageFragment `json:"fragments,omitempty"`
	SeqNo     int64              `json:"seq_no"`

	// Extra
	// source doc id citations
	Citations []valobj.Id `json:"citations,omitempty"`

	// fields for internal use
	taskId       valobj.Id          `json:"-"`
	streamEvents []*StreamTaskEvent `json:"-"`
}

func newMessage(chatId, taskId valobj.Id, userId string, role MessageRole) *Message {
	return &Message{
		Base:   entity.NewBase(),
		ChatId: chatId,
		UserId: userId,
		Role:   role,
		taskId: taskId,
		SeqNo:  time.Now().UnixNano(),
	}
}

func NewUserTextMessage(chatId, taskId valobj.Id, userId string, question string) *Message {
	msg := newMessage(chatId, taskId, userId, MessageRoleUser)
	msg.Fragments = append(msg.Fragments, NewMessageFragmentRequest(msg.lastFragmentId()+1, question))
	return msg
}

func NewAssistantMessage(chatId, taskId valobj.Id, userId string) *Message {
	msg := newMessage(chatId, taskId, userId, MessageRoleAssistant)
	msg.streamEvents = append(msg.streamEvents, &StreamTaskEvent{
		Id:         idgen.Get(taskId.String()),
		TaskId:     taskId,
		CreateTime: valobj.NewTime().Value(),
		Action:     EventActionInit,
		Path:       EventTargetPathMessage,
		Message:    msg,
	})
	return msg
}

func (m *Message) lastFragmentId() int64 {
	if len(m.Fragments) == 0 {
		return 0
	}

	return m.Fragments[len(m.Fragments)-1].Id
}

func (m *Message) lastFragment() *MessageFragment {
	if len(m.Fragments) == 0 {
		return nil
	}

	return m.Fragments[len(m.Fragments)-1]
}

func (m *Message) GetFragmentBytes() ([]byte, error) {
	return sonic.Marshal(m.Fragments)
}

func (m *Message) SetFragmentsFromBytes(data []byte) error {
	return sonic.Unmarshal(data, &m.Fragments)
}

func (m *Message) BeginThinkFragment() {
	m.Fragments = append(m.Fragments, NewMessageFragmentThink(m.lastFragmentId()+1, ""))
	m.streamEvents = append(m.streamEvents, &StreamTaskEvent{
		Id:         idgen.Get(m.taskId.String()),
		TaskId:     m.taskId,
		CreateTime: valobj.NewTime().Value(),
		Action:     EventActionNew,
		Path:       EventTargetPathFragmentThink,
		Index:      -1,
		Think:      &EventThink{Status: FragmentStatusRunning},
	})
}

func (m *Message) EndThinkFragment() {
	if m.lastFragment().Type != FragmentTypeThink {
		return
	}

	m.lastFragment().Think.Status = FragmentStatusFinished
	m.streamEvents = append(m.streamEvents, &StreamTaskEvent{
		Id:         idgen.Get(m.taskId.String()),
		TaskId:     m.taskId,
		CreateTime: valobj.NewTime().Value(),
		Action:     EventActionSet,
		Path:       EventTargetPathFragmentThinkStatus,
		Index:      -1,
		Think:      &EventThink{Status: FragmentStatusFinished},
	})
}

func (m *Message) AppendThinkFragment(s string) {
	if m.lastFragment().Type != FragmentTypeThink {
		return
	}

	m.lastFragment().Think.Append(s)
	m.streamEvents = append(m.streamEvents, &StreamTaskEvent{
		Id:         idgen.Get(m.taskId.String()),
		TaskId:     m.taskId,
		CreateTime: valobj.NewTime().Value(),
		Action:     EventActionAppend,
		Path:       EventTargetPathFragmentThinkContent,
		Index:      -1,
		Think:      &EventThink{Content: s},
	})
}

func (m *Message) BeginResponseFragment() {
	m.Fragments = append(m.Fragments, NewMessageFragmentResponse(m.lastFragmentId()+1, ""))
	m.streamEvents = append(m.streamEvents, &StreamTaskEvent{
		Id:         idgen.Get(m.taskId.String()),
		TaskId:     m.taskId,
		CreateTime: valobj.NewTime().Value(),
		Action:     EventActionNew,
		Path:       EventTargetPathFragmentResponse,
		Index:      -1,
		Response:   &EventResponse{Status: FragmentStatusRunning},
	})
}

func (m *Message) EndResponseFragment() {
	if m.lastFragment().Type != FragmentTypeResponse {
		return
	}

	m.lastFragment().Response.Status = FragmentStatusFinished
	m.streamEvents = append(m.streamEvents, &StreamTaskEvent{
		Id:         idgen.Get(m.taskId.String()),
		TaskId:     m.taskId,
		CreateTime: valobj.NewTime().Value(),
		Action:     EventActionSet,
		Path:       EventTargetPathFragmentResponseStatus,
		Index:      -1,
		Response:   &EventResponse{Status: FragmentStatusFinished},
	})
}

func (m *Message) AppendResponseFragment(s string) {
	if m.lastFragment().Type != FragmentTypeResponse {
		return
	}

	m.lastFragment().Response.AppendText(s)
	m.streamEvents = append(m.streamEvents, &StreamTaskEvent{
		Id:         idgen.Get(m.taskId.String()),
		TaskId:     m.taskId,
		CreateTime: valobj.NewTime().Value(),
		Action:     EventActionAppend,
		Path:       EventTargetPathFragmentResponseContentText,
		Index:      -1,
		Response: &EventResponse{
			Content: &FragmentContentUnion{
				Type: FragmentContentTypeText,
				Text: NewFragmentContentUnionText(s),
			},
		},
	})
}

func (m *Message) BeginPhaseFragment(summary, thought string) {
	m.Fragments = append(m.Fragments, NewMessageFragmentPhase(m.lastFragmentId()+1, summary, thought))
	m.streamEvents = append(m.streamEvents, &StreamTaskEvent{
		Id:         idgen.Get(m.taskId.String()),
		TaskId:     m.taskId,
		CreateTime: valobj.NewTime().Value(),
		Action:     EventActionNew,
		Path:       EventTargetPathFragmentPhase,
		Index:      -1,
		Phase: &EventPhase{
			Phase: &FragmentPhase{
				Status:  FragmentStatusFinished,
				Summary: summary,
				Thought: thought,
			},
		},
	})
}

func (m *Message) SetCitations(citations []valobj.Id) {
	m.Citations = citations
	m.streamEvents = append(m.streamEvents, &StreamTaskEvent{
		Id:         idgen.Get(m.taskId.String()),
		TaskId:     m.taskId,
		CreateTime: valobj.NewTime().Value(),
		Action:     EventActionSet,
		Path:       EventTargetPathCitations,
		Citations:  m.Citations,
	})
}

func (m *Message) ConsumeEvents() []*StreamTaskEvent {
	events := m.streamEvents
	m.streamEvents = m.streamEvents[:0] // clear the event slices
	return events
}

// Fragment Definitions

type FragmentType string

const (
	FragmentTypeRequest  FragmentType = "REQUEST"
	FragmentTypeThink    FragmentType = "THINK"
	FragmentTypePhase    FragmentType = "PHASE" // 类似agent创建todo的功能
	FragmentTypeResponse FragmentType = "RESPONSE"
)

func (t FragmentType) String() string { return string(t) }

type FragmentStatus string

const (
	FragmentStatusRunning  FragmentStatus = "RUNNING"
	FragmentStatusFinished FragmentStatus = "FINISHED"
)

type MessageFragment struct {
	Id   int64        `json:"id"`
	Type FragmentType `json:"type"`

	// oneof the following according to Type
	// for user message
	Request *FragmentRequest `json:"request,omitempty"`

	// for assistant message
	Think    *FragmentThink    `json:"think,omitempty"`
	Response *FragmentResponse `json:"response,omitempty"`
	Phase    *FragmentPhase    `json:"phase,omitempty"`
}

func NewMessageFragmentRequest(id int64, s string) *MessageFragment {
	return &MessageFragment{
		Id:   id,
		Type: FragmentTypeRequest,
		Request: &FragmentRequest{Content: &FragmentContentUnion{
			Type: FragmentContentTypeText,
			Text: NewFragmentContentUnionText(s),
		}},
	}
}

func NewMessageFragmentThink(id int64, s string) *MessageFragment {
	return &MessageFragment{
		Id:   id,
		Type: FragmentTypeThink,
		Think: &FragmentThink{
			Status:  FragmentStatusRunning,
			Content: NewFragmentContentUnionText(s),
		},
	}
}

func NewMessageFragmentResponse(id int64, s string) *MessageFragment {
	return &MessageFragment{
		Id:   id,
		Type: FragmentTypeResponse,
		Response: &FragmentResponse{
			Status: FragmentStatusRunning,
			Content: &FragmentContentUnion{
				Type: FragmentContentTypeText,
				Text: NewFragmentContentUnionText(s),
			},
		},
	}
}

func NewMessageFragmentPhase(id int64, summary, thought string) *MessageFragment {
	return &MessageFragment{
		Id:   id,
		Type: FragmentTypePhase,
		Phase: &FragmentPhase{
			Status:  FragmentStatusFinished, // 只有一个瞬间的状态
			Summary: summary,
			Thought: thought,
		},
	}
}

func (f *MessageFragment) AppendText(s string) {
	switch f.Type {
	case FragmentTypeRequest:
		f.Request.AppendText(s)
	case FragmentTypeThink:
		f.Think.Append(s)
	case FragmentTypeResponse:
		f.Response.AppendText(s)
	default:
		// do nothing
	}
}

// Fragment Variants definitions

type FragmentRequest struct {
	Content *FragmentContentUnion `json:"content"`
}

func (f *FragmentRequest) AppendText(s string) {
	f.Content.AppendText(s)
}

type FragmentThink struct {
	Status  FragmentStatus            `json:"status"`
	Content *FragmentContentUnionText `json:"content"`
}

func (t *FragmentThink) Append(s string) {
	t.Content.Append(s)
}

type FragmentResponse struct {
	Status  FragmentStatus        `json:"status"`
	Content *FragmentContentUnion `json:"content"`
}

func (t *FragmentResponse) AppendText(s string) {
	t.Content.AppendText(s)
}

type FragmentPhase struct {
	Status  FragmentStatus `json:"status"`
	Summary string         `json:"summary"`
	Thought string         `json:"thought"`
}

// Fragment Content Variants definitions

type FragmentContentType string

const (
	FragmentContentTypeText FragmentContentType = "text"
)

func (t FragmentContentType) String() string {
	return string(t)
}

type FragmentContentUnion struct {
	Type FragmentContentType `json:"type"`

	// one of the following according to Type
	Text *FragmentContentUnionText `json:"text"`
}

func (t *FragmentContentUnion) AppendText(s string) {
	switch t.Type {
	case FragmentContentTypeText:
		t.Text.Append(s)
	default:
		// do nothing
	}
}

// Fragment Content Union Text definitions

type FragmentContentUnionText struct {
	builder *strings.Builder
}

func NewFragmentContentUnionText(text string) *FragmentContentUnionText {
	unionText := &FragmentContentUnionText{
		builder: &strings.Builder{},
	}

	unionText.Append(text)
	return unionText
}

func (t *FragmentContentUnionText) Append(s string) {
	t.builder.WriteString(s)
}

func (t *FragmentContentUnionText) Content() string {
	return t.builder.String()
}

type fragmentContentTextAlias struct {
	Content string `json:"content"`
}

func (t *FragmentContentUnionText) MarshalJSON() ([]byte, error) {
	if t == nil {
		return []byte("null"), nil
	}

	return sonic.Marshal(fragmentContentTextAlias{Content: t.Content()})
}

func (t *FragmentContentUnionText) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("FragmentContentText is nil")
	}

	var alias fragmentContentTextAlias
	if err := sonic.Unmarshal(data, &alias); err != nil {
		return err
	}

	if t.builder == nil {
		t.builder = &strings.Builder{}
	} else {
		t.builder.Reset()
	}

	t.builder.WriteString(alias.Content)
	return nil
}
