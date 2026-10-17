package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type validateSection struct {
	Host string `toml:"host" validate:"required"`
	Port int    `toml:"port" validate:"required"`
}

type validateConfig struct {
	Name     string          `toml:"name"    validate:"required"`
	Level    string          `toml:"level"   validate:"oneof=debug info warn"`
	Retries  int             `toml:"retries" validate:"min=1"`
	Optional validateSection `toml:"optional" validate:"omitempty"`
	Extra    validateSection `toml:"extra"`
}

func validConfig() *validateConfig {
	return &validateConfig{
		Name:    "gonotelm",
		Level:   "info",
		Retries: 3,
		Extra:   validateSection{Host: "127.0.0.1", Port: 5432},
	}
}

func TestValidate(t *testing.T) {
	t.Run("passes when every tagged field is valid", func(t *testing.T) {
		require.NoError(t, Validate(validConfig()))
	})

	t.Run("reports missing fields by toml key", func(t *testing.T) {
		err := Validate(&validateConfig{})
		require.Error(t, err)

		assert.Contains(t, err.Error(), "name: required")
		assert.Contains(t, err.Error(), "extra.host: required")
		assert.Contains(t, err.Error(), "extra.port: required")
	})

	t.Run("supports every validator rule, not only required", func(t *testing.T) {
		cfg := validConfig()
		cfg.Level = "verbose"
		cfg.Retries = 0

		err := Validate(cfg)
		require.Error(t, err)

		assert.Contains(t, err.Error(), "level: oneof=debug info warn")
		assert.Contains(t, err.Error(), "retries: min=1")
	})

	t.Run("skips an unset omitempty section", func(t *testing.T) {
		require.NoError(t, Validate(validConfig()))
	})

	t.Run("validates a populated omitempty section", func(t *testing.T) {
		cfg := validConfig()
		// 段落非零值就该完整校验：只填了 host，port 必须报错
		cfg.Optional = validateSection{Host: "127.0.0.1"}

		err := Validate(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "optional.port: required")
	})
}
