package chat

import "github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"

type MessageStreamPhaseType string

const (
	MessageStreamPhaseRetrieving MessageStreamPhaseType = "retrieving"
	MessageStreamPhaseThinking   MessageStreamPhaseType = "thinking"
	MessageStreamPhaseAnswer     MessageStreamPhaseType = "answer"
)

type MessageStreamPhaseStatus string

// 数据流转阶段 一般都是 typing -> finished
// finished之后跳转到下一个phase 比如从thinking -> answer
const (
	MessageStreamTyping   MessageStreamPhaseStatus = "typing"   // 正在处理中
	MessageStreamFinished MessageStreamPhaseStatus = "finished" // 输出完成
)

type MessageStreamPhaseData struct {
	Type    MessageStreamPhaseType   `json:"type"`
	Status  MessageStreamPhaseStatus `json:"status"`
	Content string                   `json:"content,omitempty"`

	Citation *PhaseCitation `json:"citation,omitempty"` // 对应retrieving阶段的引用文档信息
}

type PhaseCitation struct {
	Items []*PhaseCitationItem `json:"items,omitempty"`
}

type PhaseCitationItem struct {
	SourceId string                    `json:"source_id,omitempty"` // 来源id
	DocId    string                    `json:"doc_id,omitempty"`    // 文档id
	Position *PhaseCitationDocPosition `json:"position,omitempty"`
}

type PhaseCitationDocPosition struct {
	// TODO 引用的文档片段在source中的定位位置 方便前端预览时跳转
	Start int `json:"start"`
	End   int `json:"end"`
}

func (p *MessageStreamPhaseData) AppendCitationItem(item *PhaseCitationItem) {
	if p.Citation == nil {
		p.Citation = &PhaseCitation{}
	}
	p.Citation.Items = append(p.Citation.Items, item)
}

type FinishReason string

const (
	FinishReasonStop          FinishReason = "stop"
	FinishReasonLength        FinishReason = "length"
	FinishReasonContentFilter FinishReason = "content_filter"
)

const MessageStreamEventHeartbeat = "heartbeat"

// 流式输出
type MessageStreamEvent struct {
	Id           int64                    `json:"id"`
	Heartbeat    string                   `json:"heartbeat,omitempty"` // heartbeat frame
	Phase        *MessageStreamPhaseData  `json:"phase,omitempty"`
	Finished     bool                     `json:"finished,omitempty"` // 流式输出是否完成 最后一条消息时为true
	FinishReason FinishReason             `json:"finish_reason,omitempty"`
	Timestamp    int64                    `json:"timestamp"` // unix timestamp
	Extra        *MessageStreamEventExtra `json:"extra,omitempty"`

	StreamId string `json:"stream_id,omitempty"`
}

type MessageStreamEventExtra struct {
	// extra fields if needed
}

type MessageStreamTaskStatus string

const (
	MessageStreamTaskStatusRunning  MessageStreamTaskStatus = "running"
	MessageStreamTaskStatusFinished MessageStreamTaskStatus = "finished"
	MessageStreamTaskStatusAborted  MessageStreamTaskStatus = "aborted"
)

func (s MessageStreamTaskStatus) String() string {
	return string(s)
}

func (s MessageStreamTaskStatus) IsRunning() bool {
	return s == MessageStreamTaskStatusRunning
}

func (s MessageStreamTaskStatus) IsFinished() bool {
	return s == MessageStreamTaskStatusFinished
}

func (s MessageStreamTaskStatus) IsAborted() bool {
	return s == MessageStreamTaskStatusAborted
}

type MessageStreamTask struct {
	Id        string
	Status    MessageStreamTaskStatus
	ChatId    string
	UserId    string
	CreatedAt int64
}

func NewMessageStreamTask(task *schema.ChatMessageTask) (*MessageStreamTask, error) {
	return &MessageStreamTask{
		Id:        task.Id,
		Status:    MessageStreamTaskStatus(task.Status),
		ChatId:    task.ChatId,
		UserId:    task.UserId,
		CreatedAt: task.CreatedAt,
	}, nil
}
