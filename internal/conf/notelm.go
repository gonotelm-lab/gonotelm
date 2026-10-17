package conf

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/conf/shared"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	rerank "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/rerank"
	mqimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	"github.com/gonotelm-lab/gonotelm/pkg/idp"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"
)

var notelmGlobal *NotelmConfig

type NotelmConfig struct {
	shared.InfraConfig

	DeployEnv string `toml:"deployEnv"`

	Api       ApiConfig            `toml:"api"`
	Cors      CorsConfig           `toml:"cors"`
	Auth      AuthConfig           `toml:"auth"`
	Mailer    MailerConfig         `toml:"mailer"`
	Chat      ChatConfig           `toml:"chat"`
	Source    SourceConfig         `toml:"source"`
	Rerank    rerank.RerankConfig  `toml:"rerank"`
	Logging   shared.LoggingConfig `toml:"logging"`
	Chunking  ChunkingConfig       `toml:"chunking"`
	Flow      shared.FlowConfig    `toml:"flow"`
	Syncer    SyncerConfig         `toml:"syncer"`
	Cleanup   CleanupConfig        `toml:"cleanup"`
	OtelTrace trace.Config         `toml:"otelTrace"`
	IDP       map[string]IDPConfig `toml:"idp"`
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

// 删除后的清理逻辑配置
type CleanupConfig struct {
	BaseURL string        `toml:"baseURL" validate:"required"`
	Delay   time.Duration `toml:"delay"`
	Jitter  time.Duration `toml:"jitter"`
}

func (c *CleanupConfig) Init() {
	if c.Delay <= 0 {
		c.Delay = 30 * time.Minute
	}
	if c.Jitter <= 0 {
		c.Jitter = 10 * time.Minute
	}
}

func (c *NotelmConfig) IsDev() bool {
	return shared.IsDevEnv(c.DeployEnv)
}

func (c *NotelmConfig) ToIDPConfig() map[string]idp.Config {
	configs := make(map[string]idp.Config)
	for k, v := range c.IDP {
		configs[k] = idp.Config{
			ClientID:     v.ClientId,
			ClientSecret: v.ClientSecret,
			RedirectURL:  v.RedirectUri,
			Scopes:       v.Scopes,
			AuthURL:      v.AuthEndpoint,
			TokenURL:     v.TokenEndpoint,
		}
	}
	return configs
}

type ApiConfig struct {
	Port            int           `toml:"port"            validate:"required"`
	ExitWaitTimeout time.Duration `toml:"exitWaitTimeout"`
}

func (c *ApiConfig) HostPort() string {
	return fmt.Sprintf(":%d", c.Port)
}

type CorsConfig struct {
	AllowOrigins []string `toml:"allowOrigins"`
}

func (c *CorsConfig) Init() {
	origins := make([]string, 0, len(c.AllowOrigins))
	for _, raw := range c.AllowOrigins {
		for origin := range strings.SplitSeq(raw, ",") {
			if origin = strings.TrimSpace(origin); origin != "" {
				origins = append(origins, origin)
			}
		}
	}
	c.AllowOrigins = origins
}

type AuthConfig struct {
	// ReturnToAllowOrigins 登录后 return_to 允许跳转的站外来源白名单
	ReturnToAllowOrigins []string `toml:"returnToAllowOrigins"`

	// EmailVerificationRequired 注册是否必须完成邮箱验证，false 时邮箱视为已验证
	EmailVerificationRequired bool `toml:"emailVerificationRequired"`

	// RequireComplexPassword 密码是否必须同时含大写字母、小写字母与数字
	RequireComplexPassword bool `toml:"requireComplexPassword"`

	// PasswordResetURLBase 找回密码邮件里重置链接的前端页面，最终链接为 base#token=xxx
	PasswordResetURLBase string `toml:"passwordResetUrlBase"`

	// LoginErrorURL 三方登录回调失败时的前端页面
	LoginErrorURL string `toml:"loginErrorUrl"`

	// BindErrorURL 三方绑定回调失败时的前端页面
	BindErrorURL string `toml:"bindErrorUrl"`
}

func (c *AuthConfig) Init() {
	origins := make([]string, 0, len(c.ReturnToAllowOrigins))
	for _, raw := range c.ReturnToAllowOrigins {
		for origin := range strings.SplitSeq(raw, ",") {
			if origin = strings.TrimSpace(origin); origin != "" {
				origins = append(origins, origin)
			}
		}
	}
	c.ReturnToAllowOrigins = origins
}

// MailerConfig 事务性邮件的公共配置，type 选具体 sender，构建失败则用 log
type MailerConfig struct {
	Type     string        `toml:"type"`     // resend | log
	FromName string        `toml:"fromName"` // display name, shared by every sender
	Timeout  time.Duration `toml:"timeout"`

	Resend ResendMailerConfig `toml:"resend"`
	Log    LogMailerConfig    `toml:"log"`
}

// ResendMailerConfig Resend credentials, provider decides which addresses can be From
type ResendMailerConfig struct {
	APIKey string `toml:"apiKey"`
	From   string `toml:"from"`
}

type LogMailerConfig struct {
	LogLevel string `toml:"logLevel"`
}

func (c *MailerConfig) Init() {
	if c.Type == "" {
		c.Type = "log"
	}
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
}

// Level slog level of the log mailer, defaults to info
func (c *LogMailerConfig) Level() slog.Level {
	if strings.TrimSpace(c.LogLevel) == "" {
		return slog.LevelInfo
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(c.LogLevel)); err != nil {
		return slog.LevelInfo
	}

	return level
}

type IDPConfig struct {
	ClientId      string   `toml:"clientId"`
	ClientSecret  string   `toml:"clientSecret"`
	RedirectUri   string   `toml:"redirectUri"`
	Scopes        []string `toml:"scopes"`
	AuthEndpoint  string   `toml:"authEndpoint"`
	TokenEndpoint string   `toml:"tokenEndpoint"`
}

func LoadNotelmConfig(path string) (*NotelmConfig, error) {
	cfg := &NotelmConfig{}
	if err := shared.LoadTOML(path, cfg); err != nil {
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
	c.Cleanup.Init()
	c.Cors.Init()
	c.Auth.Init()
	c.Mailer.Init()

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
