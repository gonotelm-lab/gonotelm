package conf

import (
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/conf/shared"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	rerank "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/rerank"
	mqimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"

	"github.com/BurntSushi/toml"
	"github.com/a8m/envsubst"
)

var notelmGlobal *NotelmConfig

type NotelmConfig struct {
	shared.InfraConfig

	DeployEnv string `toml:"deployEnv"`

	Api       ApiConfig            `toml:"api"`
	Chat      ChatConfig           `toml:"chat"`
	Source    SourceConfig         `toml:"source"`
	Rerank    rerank.RerankConfig  `toml:"rerank"`
	Logging   shared.LoggingConfig `toml:"logging"`
	Chunking  ChunkingConfig       `toml:"chunking"`
	Flow      shared.FlowConfig    `toml:"flow"`
	Syncer    SyncerConfig         `toml:"syncer"`
	OtelTrace trace.Config         `toml:"otelTrace"`
}

var sourceUrlBlacklistRegex *regexp.Regexp

type SourceConfig struct {
	ModelProvider     llmchat.Provider `toml:"modelProvider"`
	Model             string           `toml:"model"`
	UrlBlacklistRegex string           `toml:"urlBlacklistRegex"`
}

func (c *SourceConfig) init() {
	sync.OnceFunc(func() {
		sourceUrlBlacklistRegex = regexp.MustCompile(c.UrlBlacklistRegex)
	})()
}

func (c *SourceConfig) GetURLBlacklistRegex() *regexp.Regexp {
	return sourceUrlBlacklistRegex
}

type ChatConfig struct {
	MaxRound              int                   `toml:"maxRound"`
	ModelProvider         llmchat.Provider      `toml:"modelProvider"`
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

type SyncerConfig struct {
	PerTaskInterval time.Duration `toml:"perTaskInterval"`
	GlobalInterval  time.Duration `toml:"globalInterval"`
	GlobalBatchSize int           `toml:"globalBatchSize"`
}

func (c *NotelmConfig) IsDev() bool {
	return shared.IsDevEnv(c.DeployEnv)
}

func (c *WorkerConfig) IsDev() bool {
	return shared.IsDevEnv(c.DeployEnv)
}

type ApiConfig struct {
	Port            int           `toml:"port"`
	ExitWaitTimeout time.Duration `toml:"exitWaitTimeout"`
}

func (c *ApiConfig) HostPort() string {
	return fmt.Sprintf(":%d", c.Port)
}

func LoadTOML(path string, cfg interface{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file %q failed: %w", path, err)
	}

	expanded, err := envsubst.String(string(raw))
	if err != nil {
		return fmt.Errorf("expand env in config file %q failed: %w", path, err)
	}

	if _, err := toml.Decode(expanded, cfg); err != nil {
		return fmt.Errorf("decode config file %q failed: %w", path, err)
	}

	return nil
}

func LoadNotelmConfig(path string) (*NotelmConfig, error) {
	cfg := &NotelmConfig{}
	if err := LoadTOML(path, cfg); err != nil {
		return nil, err
	}

	cfg.init()

	notelmGlobal = cfg
	notelmGlobal.Source.init()

	return notelmGlobal, nil
}

func (c *NotelmConfig) init() {
	c.InitInfra()
	c.Logging.Init()
	c.Flow.Init()

	if c.MessageQueue.Type == "" {
		c.MessageQueue.Type = mqimpl.Kafka
	}
	if c.Syncer.PerTaskInterval == 0 {
		c.Syncer.PerTaskInterval = 2 * time.Second
	}
	if c.Syncer.GlobalInterval == 0 {
		c.Syncer.GlobalInterval = 5 * time.Second
	}
	if c.Syncer.GlobalBatchSize <= 0 {
		c.Syncer.GlobalBatchSize = 100
	}
}

func NotelmGlobal() *NotelmConfig {
	return notelmGlobal
}
