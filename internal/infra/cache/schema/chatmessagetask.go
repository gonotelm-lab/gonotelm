package schema

type ChatMessageTask struct {
	Id        string `json:"id"         msgpack:"id"`         // 任务id
	Status    string `json:"status"     msgpack:"status"`     // 任务状态
	CreatedAt int64  `json:"created_at" msgpack:"created_at"` // 任务创建时间
	ChatId    string `json:"chat_id"    msgpack:"chat_id"`    // 会话id
	UserId    string `json:"user_id"    msgpack:"user_id"`    // 用户id
}

type ChatMessageTaskEvent struct {
	Data []byte `json:"data" msgpack:"data"` // 事件数据
}
