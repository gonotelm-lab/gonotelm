package conf

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const flowWorkerSyncerToml = `
[flow]
addr        = "flow.example:9443"
namespace   = "gonotelm"
maxRetry    = 3
dialTimeout = "5s"

[syncer]
perTaskInterval = "2s"
globalInterval   = "5s"
globalBatchSize  = 100

[worker]
name            = "gonotelm-worker-1"
maxConcurrency  = 4
heartbeat       = "5s"
taskTypes       = ["artifact.mindmap", "artifact.report", "artifact.info_graphic", "artifact.audio_overview"]
`

func TestLoad_FlowWorkerSyncer(t *testing.T) {
	cfg := &Config{}
	_, err := toml.Decode(flowWorkerSyncerToml, cfg)
	require.NoError(t, err)

	assert.Equal(t, "flow.example:9443", cfg.Flow.Addr)
	assert.Equal(t, "gonotelm", cfg.Flow.Namespace)
	assert.Equal(t, 3, cfg.Flow.MaxRetry)
	assert.Equal(t, 5*time.Second, cfg.Flow.DialTimeout)

	assert.Equal(t, 4, cfg.Worker.MaxConcurrency)
	assert.Equal(t, 5*time.Second, cfg.Worker.Heartbeat)
	assert.Len(t, cfg.Worker.TaskTypes, 4)

	assert.Equal(t, 2*time.Second, cfg.Syncer.PerTaskInterval)
	assert.Equal(t, 5*time.Second, cfg.Syncer.GlobalInterval)
	assert.Equal(t, 100, cfg.Syncer.GlobalBatchSize)
}

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.toml")
	require.NoError(t, os.WriteFile(path, []byte(``), 0644))

	cfg, err := Load(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 3, cfg.Flow.MaxRetry)
	assert.Equal(t, 5*time.Second, cfg.Flow.DialTimeout)
	assert.Equal(t, 2*time.Second, cfg.Syncer.PerTaskInterval)
	assert.Equal(t, 5*time.Second, cfg.Syncer.GlobalInterval)
	assert.Equal(t, 100, cfg.Syncer.GlobalBatchSize)
	assert.Equal(t, 4, cfg.Worker.MaxConcurrency)
	assert.Equal(t, 5*time.Second, cfg.Worker.Heartbeat)
}

func TestLoad_TemplateParses(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "etc", "gonotelm.toml.tpl"))
	require.NoError(t, err)
	require.NotNil(t, cfg)
}