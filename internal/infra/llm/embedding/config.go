package embedding

import "time"

type Type string

const (
	Ark          Type = "ark"
	DashScope    Type = "dashscope"
	Gemini       Type = "gemini"
	Ollama       Type = "ollama"
	OpenAI       Type = "openai"
	Qianfan      Type = "qianfan"
	TencentCloud Type = "tencentcloud"
)

type Config struct {
	Type Type `toml:"type"`
	// BatchSize controls max text count per embedding request.
	BatchSize int `toml:"batchSize"`
	// MaxConcurrency controls max concurrent embedding batch requests.
	MaxConcurrency int `toml:"maxConcurrency"`

	Ark          ArkConfig          `toml:"ark"`
	DashScope    DashScopeConfig    `toml:"dashscope"`
	Gemini       GeminiConfig       `toml:"gemini"`
	Ollama       OllamaConfig       `toml:"ollama"`
	OpenAI       OpenAIConfig       `toml:"openai"`
	Qianfan      QianfanConfig      `toml:"qianfan"`
	TencentCloud TencentCloudConfig `toml:"tencentcloud"`
}

type ArkConfig struct {
	Timeout               *time.Duration `toml:"timeout"`
	RetryTimes            *int           `toml:"retryTimes"`
	BaseURL               string         `toml:"baseUrl"`
	Region                string         `toml:"region"`
	APIKey                string         `toml:"apiKey"`
	AccessKey             string         `toml:"accessKey"`
	SecretKey             string         `toml:"secretKey"`
	Model                 string         `toml:"model"`
	APIType               string         `toml:"apiType"`
	MaxConcurrentRequests *int           `toml:"maxConcurrentRequests"`
}

type DashScopeConfig struct {
	APIKey     string        `toml:"apiKey"`
	Timeout    time.Duration `toml:"timeout"`
	Model      string        `toml:"model"`
	Dimensions *int          `toml:"dimensions"`
}

type GeminiConfig struct {
	APIKey               string `toml:"apiKey"`
	Backend              string `toml:"backend"`
	Project              string `toml:"project"`
	Location             string `toml:"location"`
	Model                string `toml:"model"`
	TaskType             string `toml:"taskType"`
	Title                string `toml:"title"`
	OutputDimensionality *int32 `toml:"outputDimensionality"`
	MIMEType             string `toml:"mimeType"`
	AutoTruncate         bool   `toml:"autoTruncate"`
}

type OllamaConfig struct {
	Timeout   time.Duration  `toml:"timeout"`
	BaseURL   string         `toml:"baseUrl"`
	Model     string         `toml:"model"`
	Truncate  *bool          `toml:"truncate"`
	KeepAlive *time.Duration `toml:"keepAlive"`
	Options   map[string]any `toml:"options"`
}

type OpenAIConfig struct {
	Timeout        time.Duration `toml:"timeout"`
	APIKey         string        `toml:"apiKey"`
	ByAzure        bool          `toml:"byAzure"`
	BaseURL        string        `toml:"baseUrl"`
	APIVersion     string        `toml:"apiVersion"`
	Model          string        `toml:"model"`
	EncodingFormat string        `toml:"encodingFormat"`
	Dimensions     *int          `toml:"dimensions"`
	User           string        `toml:"user"`
}

type QianfanConfig struct {
	AK                    string   `toml:"ak"`
	SK                    string   `toml:"sk"`
	AccessKey             string   `toml:"accessKey"`
	SecretKey             string   `toml:"secretKey"`
	AccessToken           string   `toml:"accessToken"`
	BearerToken           string   `toml:"bearerToken"`
	Model                 string   `toml:"model"`
	LLMRetryCount         *int     `toml:"llmRetryCount"`
	LLMRetryTimeout       *float32 `toml:"llmRetryTimeout"`
	LLMRetryBackoffFactor *float32 `toml:"llmRetryBackoffFactor"`
}

type TencentCloudConfig struct {
	SecretID  string `toml:"secretId"`
	SecretKey string `toml:"secretKey"`
	Region    string `toml:"region"`
}
