package conf

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
)

const (
	DefaultSourceDocsRecallCount = 30
	DefaultTaskTimeout           = 5 * time.Minute
)

type LogicConfig struct {
	Chat ChatLogicConfig `toml:"chat"`
}

type ChatLogicConfig struct {
	ModelProvider         chat.Type     `toml:"modelProvider"`
	SourceDocsRecallCount int           `toml:"sourceDocsRecallCount"`
	TaskTimeout           time.Duration `toml:"taskTimeout"` // 流式任务超时时间
}

func (c *ChatLogicConfig) GetSourceDocsRecallCount() int {
	if c.SourceDocsRecallCount == 0 {
		return DefaultSourceDocsRecallCount
	}

	return c.SourceDocsRecallCount
}

func (c *ChatLogicConfig) GetTaskTimeout() time.Duration {
	if c.TaskTimeout == 0 {
		return DefaultTaskTimeout
	}

	return c.TaskTimeout
}
