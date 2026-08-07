package conf

import (
	"regexp"
	"sync"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/conf/shared"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	rerank "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/rerank"
	text2audio "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	text2image "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"
)

var workerGlobal *WorkerConfig

const (
	DefaultMaxRound              = 30
	DefaultSourceDocsRecallCount = 50
	DefaultTaskTimeout           = 5 * time.Minute
	DefaultGMMAutoMaxClusters    = 50
	RerankDefaultTopN            = 20
)

type WorkerConfig struct {
	shared.InfraConfig

	DeployEnv string               `toml:"deployEnv"`
	Studio    StudioConfig         `toml:"studio"`
	Logging   shared.LoggingConfig `toml:"logging"`
	Flow      shared.FlowConfig    `toml:"flow"`
	Worker    WorkerPoolConfig     `toml:"worker"`
	OtelTrace trace.Config         `toml:"otelTrace"`
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

var sourceUrlBlacklistRegex *regexp.Regexp

type SourceConfig struct {
	ModelProvider llm.Provider `toml:"modelProvider"`
	Model         string       `toml:"model"`

	UrlBlacklistRegex string `toml:"urlBlacklistRegex"`

	BizCache struct {
		Eviction time.Duration `toml:"eviction"`
		MaxMB    int           `toml:"maxMB"`
	} `toml:"bizCache"`
}

func (c *SourceConfig) init() {
	sync.OnceFunc(func() {
		sourceUrlBlacklistRegex = regexp.MustCompile(c.UrlBlacklistRegex)
	})()
}

func (c *SourceConfig) GetURLBlacklistRegex() *regexp.Regexp {
	return sourceUrlBlacklistRegex
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

	Flashcard struct {
		MaxRound      int          `toml:"maxRound"`
		ModelProvider llm.Provider `toml:"modelProvider"`
		Model         string       `toml:"model"`
	} `toml:"flashcard"`

	Quiz struct {
		MaxRound      int          `toml:"maxRound"`
		ModelProvider llm.Provider `toml:"modelProvider"`
		Model         string       `toml:"model"`
	} `toml:"quiz"`

	DataTable struct {
		MaxRound      int          `toml:"maxRound"`
		ModelProvider llm.Provider `toml:"modelProvider"`
		Model         string       `toml:"model"`
	} `toml:"dataTable"`

	TaskConfig struct {
		NumClaimers        int           `toml:"numClaimers"`
		ScanInterval       time.Duration `toml:"scanInterval"`
		NumOfWorkGroup     int           `toml:"numOfWorkGroup"`
		NumWorkersPerGroup int           `toml:"numWorkersPerGroup"`
	} `toml:"taskConfig"`
}

func LoadWorkerConfig(path string) (*WorkerConfig, error) {
	cfg := &WorkerConfig{}
	if err := loadTOML(path, cfg); err != nil {
		return nil, err
	}

	cfg.applyDefaults()

	workerGlobal = cfg
	return cfg, nil
}

func (c *WorkerConfig) applyDefaults() {
	c.ApplyDefaults()
	c.Logging.ApplyDefaults()
	c.Flow.ApplyDefaults()

	if c.Worker.MaxConcurrency <= 0 {
		c.Worker.MaxConcurrency = 4
	}
	if c.Worker.Heartbeat == 0 {
		c.Worker.Heartbeat = 5 * time.Second
	}
}

func WorkerGlobal() *WorkerConfig {
	return workerGlobal
}
