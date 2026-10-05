package conf

import (
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/conf/shared"
	"github.com/gonotelm-lab/gonotelm/pkg/initjob"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"
)

var initJobGlobal *InitJobConfig

// InitJobConfig is the config for cmd/initjob.
type InitJobConfig struct {
	shared.InfraConfig

	DeployEnv string               `toml:"deployEnv"`
	Logging   shared.LoggingConfig `toml:"logging"`
	Job       InitJobJobConfig     `toml:"initJob"`
	OtelTrace trace.Config         `toml:"otelTrace"`
}

// InitJobJobConfig is the [initJob] section.
type InitJobJobConfig struct {
	// Mode is the failure policy, "strict" or "loose"; empty means strict.
	Mode string `toml:"mode"`

	// ForceTasks is a comma-separated list of task ids to re-run even though
	// they already succeeded.
	ForceTasks string `toml:"forceTasks"`
}

func (c *InitJobConfig) IsDev() bool { return shared.IsDevEnv(c.DeployEnv) }

// Mode returns the configured failure policy. An empty or unknown value is
// left for NewRunner to reject, so validation lives in one place.
func (c *InitJobConfig) Mode() initjob.Mode {
	return initjob.Mode(c.Job.Mode)
}

// ForceTaskIDs splits the configured force list; empty entries are dropped
// by NewRunner.
func (c *InitJobConfig) ForceTaskIDs() []string {
	if strings.TrimSpace(c.Job.ForceTasks) == "" {
		return nil
	}
	return strings.Split(c.Job.ForceTasks, ",")
}

func LoadInitJobConfig(path string) (*InitJobConfig, error) {
	cfg := &InitJobConfig{}
	if err := shared.LoadTOML(path, cfg); err != nil {
		return nil, err
	}

	cfg.init()

	initJobGlobal = cfg
	return cfg, nil
}

func (c *InitJobConfig) init() {
	c.InitInfra()
	c.Logging.Init()
}

func InitJobGlobal() *InitJobConfig {
	return initJobGlobal
}
