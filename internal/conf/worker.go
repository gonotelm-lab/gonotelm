package conf

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/conf/shared"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	text2audio "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	text2image "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/sandbox"
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

type StudioConfig struct {
	Mindmap struct {
		MaxRound      int              `toml:"maxRound"`
		ModelProvider llmchat.Provider `toml:"modelProvider"`
		Model         string           `toml:"model"`
	} `toml:"mindmap"`

	Report struct {
		MaxRound      int              `toml:"maxRound"`
		ModelProvider llmchat.Provider `toml:"modelProvider"`
		Model         string           `toml:"model"`
	} `toml:"report"`

	InfoGraphic struct {
		MaxRound           int                           `toml:"maxRound"`
		ModelProvider      llmchat.Provider              `toml:"modelProvider"`
		Model              string                        `toml:"model"`
		ImageModelProvider text2image.Text2ImageProvider `toml:"imageModelProvider"`
		ImageModel         string                        `toml:"imageModel"`
	} `toml:"infoGraphic"`

	AudioOverview struct {
		MaxRound              int                           `toml:"maxRound"`
		ModelProvider         llmchat.Provider              `toml:"modelProvider"`
		Model                 string                        `toml:"model"`
		AudioModelProvider    text2audio.Text2AudioProvider `toml:"audioModelProvider"`
		AudioModel            string                        `toml:"audioModel"`
		AudioSynthConcurrency int                           `toml:"audioSynthConcurrency"`
	} `toml:"audioOverview"`

	Flashcard struct {
		MaxRound      int              `toml:"maxRound"`
		ModelProvider llmchat.Provider `toml:"modelProvider"`
		Model         string           `toml:"model"`
	} `toml:"flashcard"`

	Quiz struct {
		MaxRound      int              `toml:"maxRound"`
		ModelProvider llmchat.Provider `toml:"modelProvider"`
		Model         string           `toml:"model"`
	} `toml:"quiz"`

	DataTable struct {
		MaxRound      int              `toml:"maxRound"`
		ModelProvider llmchat.Provider `toml:"modelProvider"`
		Model         string           `toml:"model"`
	} `toml:"dataTable"`

	TaskConfig struct {
		NumClaimers        int           `toml:"numClaimers"`
		ScanInterval       time.Duration `toml:"scanInterval"`
		NumOfWorkGroup     int           `toml:"numOfWorkGroup"`
		NumWorkersPerGroup int           `toml:"numWorkersPerGroup"`
	} `toml:"taskConfig"`

	Slides struct {
		MaxRound         int              `toml:"maxRound"`
		ModelProvider    llmchat.Provider `toml:"modelProvider"`
		GenerateMaxRound int              `toml:"generateMaxRound"`
		Model            string           `toml:"model"`
		SandboxProvider  sandbox.Provider `toml:"sandboxProvider"`
	} `toml:"slides"`
}

func LoadWorkerConfig(path string) (*WorkerConfig, error) {
	cfg := &WorkerConfig{}
	if err := LoadTOML(path, cfg); err != nil {
		return nil, err
	}

	cfg.init()

	workerGlobal = cfg
	return cfg, nil
}

func (c *WorkerConfig) init() {
	c.InitInfra()
	c.Logging.Init()
	c.Flow.Init()

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
