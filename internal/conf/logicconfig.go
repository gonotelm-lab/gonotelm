package conf

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
)

const (
	DefaultMaxRound              = 10
	DefaultSourceDocsRecallCount = 50
	DefaultTaskTimeout           = 5 * time.Minute
	DefaultGMMAutoMaxClusters    = 50
	RerankDefaultTopN            = 20
)

type LogicConfig struct {
	Chat   ChatLogicConfig   `toml:"chat"`
	Source SourceLogicConfig `toml:"source"`
	Studio StudioLogicConfig `toml:"studio"`
}

type ChatLogicConfig struct {
	MaxRound              int             `toml:"maxRound"`
	ModelProvider         	llm.Provider   `toml:"modelProvider"`
	Model                 string          `toml:"model"` // 对话使用的模型
	SourceDocsRecallCount int             `toml:"sourceDocsRecallCount"`
	TaskTimeout           time.Duration   `toml:"taskTimeout"`    // 流式任务超时时间
	RerankProvider        	llm.RerankProvider `toml:"rerankProvider"` // 重排序提供商
	RerankEnabled         bool            `toml:"rerankEnabled"`
	RerankTopN            int             `toml:"rerankTopN"`
	RerankModel           string          `toml:"rerankModel"`
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

func (c *ChatLogicConfig) GetRerankTopN() int {
	if c.RerankTopN == 0 {
		return RerankDefaultTopN
	}

	return c.RerankTopN
}

type SourceLogicConfig struct {
	ModelProvider 	llm.Provider `toml:"modelProvider"`
	Model         string        `toml:"model"`

	BizCache struct {
		Eviction time.Duration `toml:"eviction"`
		MaxMB    int           `toml:"maxMB"`
	} `toml:"bizCache"`
}

type StudioLogicConfig struct {
	Mindmap struct {
		MaxRound      int           `toml:"maxRound"`
		ModelProvider 	llm.Provider `toml:"modelProvider"`
		Model         string        `toml:"model"`
	} `toml:"mindmap"`

	Report struct {
		MaxRound      int           `toml:"maxRound"`
		ModelProvider 	llm.Provider `toml:"modelProvider"`
		Model         string        `toml:"model"`
	} `toml:"report"`

	InfoGraphic struct {
		MaxRound           int                 `toml:"maxRound"`
		ModelProvider      	llm.Provider       `toml:"modelProvider"`
		Model              string              `toml:"model"`
		ImageModelProvider 	llm.Text2ImageProvider `toml:"imageModelProvider"`
		ImageModel         string              `toml:"imageModel"`
	} `toml:"infoGraphic"`

	AudioOverview struct {
		MaxRound      int           `toml:"maxRound"`
		ModelProvider 	llm.Provider `toml:"modelProvider"`
		Model         string        `toml:"model"`
	} `toml:"audioOverview"`

	TaskConfig struct {
		NumClaimers        int           `toml:"numClaimers"`
		ScanInterval       time.Duration `toml:"scanInterval"`
		NumOfWorkGroup     int           `toml:"numOfWorkGroup"`
		NumWorkersPerGroup int           `toml:"numWorkersPerGroup"`
	} `toml:"taskConfig"`
}
