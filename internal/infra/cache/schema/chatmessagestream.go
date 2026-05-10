package schema

import "time"

type ChatMessageTask struct {
	Id        string `json:"id"         msgpack:"id"`         // 任务id
	Status    string `json:"status"     msgpack:"status"`     // 任务状态
	CreatedAt int64  `json:"created_at" msgpack:"created_at"` // 任务创建时间
	ChatId    string `json:"chat_id"    msgpack:"chat_id"`    // 会话id
	UserId    string `json:"user_id"    msgpack:"user_id"`    // 用户id

	ExpireDuration time.Duration `json:"-" msgpack:"-"` // 任务过期时间
}

type ChatMessageStreamEvent struct {
	streamId string `json:"-" msgpack:"-"`
	Data     []byte `json:"data" msgpack:"data"` // 事件数据 具体数据定义由业务层定义
}

func (e *ChatMessageStreamEvent) SetStreamId(streamId string) {
	e.streamId = streamId
}

func (e *ChatMessageStreamEvent) StreamId() string {
	return e.streamId
}

type PullEventStreamArgs struct {
	LastId string
	Block  time.Duration
	Count  int
}
