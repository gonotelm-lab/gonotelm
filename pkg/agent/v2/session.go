package agent

import (
	"github.com/cloudwego/eino/schema"
)

// Session 是一次会话的数据载体，只允许单 goroutine 持有。
// State 保存业务状态；System 是系统提示词；Messages 是持久化上下文；Usage 是累计用量。
type Session[S any] struct {
	State    S
	System   string
	Messages []*schema.Message
	Usage    schema.TokenUsage
}

// NewSession 创建一次会话，State 由调用方提供。
func NewSession[S any](state S) *Session[S] {
	return &Session[S]{State: state}
}

// StatelessSession 是无状态 agent 的 Session 简写。
type StatelessSession = Session[NoState]

// NewStatelessSession 创建一个无状态 Session，调用方不需要构造 NoState。
func NewStatelessSession() *StatelessSession {
	return NewSession(NoState{})
}

// Append 追加消息到会话。
func (s *Session[S]) Append(msgs ...*schema.Message) {
	if len(msgs) == 0 {
		return
	}
	s.Messages = append(s.Messages, msgs...)
}

// Replace 用新的消息列表替换会话上下文，常用于上下文压缩。
func (s *Session[S]) Replace(msgs []*schema.Message) {
	s.Messages = append([]*schema.Message(nil), msgs...)
}

// Reset 清空上下文和用量，保留 State 与 System。
func (s *Session[S]) Reset() {
	s.Messages = nil
	s.Usage = schema.TokenUsage{}
}

// Clone 复制会话，供并行分支使用。
// Messages 切片会被复制，但消息本身按只读共享；State 按值复制，
// 若 State 内含指针等引用类型，调用方需要自行深拷贝。
func (s *Session[S]) Clone() *Session[S] {
	cp := *s
	cp.Messages = append([]*schema.Message(nil), s.Messages...)
	return &cp
}
