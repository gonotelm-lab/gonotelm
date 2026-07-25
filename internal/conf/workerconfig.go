package conf

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	rerank "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/rerank"
	text2audio "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	text2image "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
)

const (
	DefaultMaxRound              = 30
	DefaultSourceDocsRecallCount = 50
	DefaultTaskTimeout           = 5 * time.Minute
	DefaultGMMAutoMaxClusters    = 50
	RerankDefaultTopN            = 20
)

type WorkerConfig struct {
	InfraConfig

	DeployEnv string           `toml:"deployEnv"`
	Studio    StudioConfig     `toml:"studio"`
	Logging   LoggingConfig    `toml:"logging"`
	Flow      FlowConfig       `toml:"flow"`
	Worker    WorkerPoolConfig `toml:"worker"`
}

type WorkerPoolConfig struct {
	MaxConcurrency int           `toml:"maxConcurrency"`
	Heartbeat      time.Duration `toml:"heartbeat"`
}

type ChatConfig struct {
	MaxRound              int                   `toml:"maxRound"`
	ModelProvider         llm.Provider          `toml:"modelProvider"`
	Model                 string                `toml:"model"` // 对话使用的模型
	SourceDocsRecallCount int                   `toml:"sourceDocsRecallCount"`
	TaskTimeout           time.Duration         `toml:"taskTimeout"`    // 流式任务超时时间
	RerankProvider        rerank.RerankProvider `toml:"rerankProvider"` // 重排序提供商
	RerankEnabled         bool                  `toml:"rerankEnabled"`
	RerankTopN            int                   `toml:"rerankTopN"`
	RerankModel           string                `toml:"rerankModel"`
}

func (c *ChatConfig) GetSourceDocsRecallCount() int {
	if c.SourceDocsRecallCount == 0 {
		return DefaultSourceDocsRecallCount
	}

	return c.SourceDocsRecallCount
}

func (c *ChatConfig) GetTaskTimeout() time.Duration {
	if c.TaskTimeout == 0 {
		return DefaultTaskTimeout
	}

	return c.TaskTimeout
}

func (c *ChatConfig) GetMaxRound() int {
	if c.MaxRound == 0 {
		return DefaultMaxRound
	}

	return c.MaxRound
}

func (c *ChatConfig) GetRerankTopN() int {
	if c.RerankTopN == 0 {
		return RerankDefaultTopN
	}

	return c.RerankTopN
}

type SourceConfig struct {
	ModelProvider llm.Provider `toml:"modelProvider"`
	Model         string       `toml:"model"`

	BizCache struct {
		Eviction time.Duration `toml:"eviction"`
		MaxMB    int           `toml:"maxMB"`
	} `toml:"bizCache"`
}

type StudioConfig struct {
	Mindmap struct {
		MaxRound      int          `toml:"maxRound"`
		ModelProvider llm.Provider `toml:"modelProvider"`
		Model         string       `toml:"model"`
	} `toml:"mindmap"`

	Report struct {
		MaxRound      int          `toml:"maxRound"`
		ModelProvider llm.Provider `toml:"modelProvider"`
		Model         string       `toml:"model"`
	} `toml:"report"`

	InfoGraphic struct {
		MaxRound           int                           `toml:"maxRound"`
		ModelProvider      llm.Provider                  `toml:"modelProvider"`
		Model              string                        `toml:"model"`
		ImageModelProvider text2image.Text2ImageProvider `toml:"imageModelProvider"`
		ImageModel         string                        `toml:"imageModel"`
	} `toml:"infoGraphic"`

	AudioOverview struct {
		MaxRound              int                           `toml:"maxRound"`
		ModelProvider         llm.Provider                  `toml:"modelProvider"`
		Model                 string                        `toml:"model"`
		AudioModelProvider    text2audio.Text2AudioProvider `toml:"audioModelProvider"`
		AudioModel            string                        `toml:"audioModel"`
		AudioSynthConcurrency int                           `toml:"audioSynthConcurrency"`
	} `toml:"audioOverview"`

	TaskConfig struct {
		NumClaimers        int           `toml:"numClaimers"`
		ScanInterval       time.Duration `toml:"scanInterval"`
		NumOfWorkGroup     int           `toml:"numOfWorkGroup"`
		NumWorkersPerGroup int           `toml:"numWorkersPerGroup"`
	} `toml:"taskConfig"`
}
