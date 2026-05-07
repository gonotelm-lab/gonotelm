package model

type ChatMessageStreamPhase string

const (
	ChatMessageStreamPhaseThinking ChatMessageStreamPhase = "thinking"
	ChatMessageStreamPhaseAnswer   ChatMessageStreamPhase = "answer"
)

type ChatMessageStreamStatus string

const (
	ChatMessageStreamHeartbeat ChatMessageStreamStatus = "heartbeat" // 下发的是心跳
	ChatMessageStreamTyping    ChatMessageStreamStatus = "typing"    // 正在输出
	ChatMessageStreamFinished  ChatMessageStreamStatus = "finished"  // 输出完成
)

// 流式输出
type ChatMessageStreamEvent struct {
	Content   string                       `json:"content"` // 回答的内容
	Phase     ChatMessageStreamPhase       `json:"phase"`
	Status    ChatMessageStreamStatus      `json:"status"`
	Timestamp int64                        `json:"timestamp"` // unix timestamp
	Extra     *ChatMessageStreamEventExtra `json:"extra,omitempty"`
}

type ChatMessageStreamEventExtra struct {
	ThinkingSummary *ChatMessageStreamThinkingSummary `json:"thinking_summary,omitempty"` // 思考过程总结
}

type ChatMessageStreamThinkingSummary struct {
	Title   string `json:"title"`   // 思考过程标题
	Summary string `json:"summary"` // 思考过程总结
	Content string `json:"content"` // 思考过程内容
}
