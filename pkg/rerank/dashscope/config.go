package dashscope

import "time"

type Config struct {
	APIKey  string        `toml:"apiKey"`
	BaseURL string        `toml:"baseURL"`
	Path    string        `toml:"path"`
	Model   string        `toml:"model"`
	Timeout time.Duration `toml:"timeout"`
}
