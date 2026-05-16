package conf

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
)

const (
	DefaultMaxRound              = 10
	DefaultSourceDocsRecallCount = 30
	DefaultTaskTimeout           = 5 * time.Minute
)

type LogicConfig struct {
	Chat   ChatLogicConfig   `toml:"chat"`
	Source SourceLogicConfig `toml:"source"`
}

type ChatLogicConfig struct {
	MaxRound              int           `toml:"maxRound"`
	ModelProvider         chat.Type     `toml:"modelProvider"`
	Model                 string        `toml:"model"` // 对话使用的模型
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

func (c *ChatLogicConfig) GetMaxRound() int {
	if c.MaxRound == 0 {
		return DefaultMaxRound
	}

	return c.MaxRound
}

type SourceLogicConfig struct {
	ModelProvider chat.Type `toml:"modelProvider"`
	Model         string    `toml:"model"`
}
