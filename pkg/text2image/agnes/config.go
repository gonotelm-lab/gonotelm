package agnes

import "time"

type Config struct {
	APIKey  string        `toml:"apiKey"`
	BaseUrl string        `toml:"baseUrl"`
	Model   string        `toml:"model"`
	Timeout time.Duration `toml:"timeout"`
}
