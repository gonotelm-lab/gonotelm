package schema

// 聊天中的建议问题
type ChatSuggestion struct {
	Type      string   `json:"type"      msgpack:"type"`
	Questions []string `json:"questions" msgpack:"questions"`
}
