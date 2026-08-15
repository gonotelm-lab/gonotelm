package sandbox

import "time"

type Provider string

func (t Provider) String() string {
	return string(t)
}

const (
	ProviderOpenSandbox Provider = "opensandbox"
)

type ProviderConfig struct {
	OpenSandbox OpenSandboxConfig `toml:"opensandbox"`
}

type OpenSandboxConfig struct {
	Endpoint string        `toml:"endpoint"`
	ApiKey   string        `toml:"apiKey"`
	Timeout  time.Duration `toml:"timeout"`
}
