package opensandbox

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testEndpoint = "http://localhost:23080"
	testApiKey   = "123456"
)

func testConfig() Config {
	return Config{
		Endpoint: testEndpoint,
		ApiKey:   testApiKey,
	}
}

func TestNewInvalidEndpoint(t *testing.T) {
	cfg := testConfig()
	cfg.Endpoint = "://bad-endpoint"
	_, err := NewManager(t.Context(), cfg)
	require.Error(t, err)
}

func TestManagerLifecycle(t *testing.T) {
	m, err := NewManager(t.Context(), testConfig())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var manager repository.Manager = m

	sbx, err := manager.CreateSandbox(ctx, entity.SandboxKey{
		UserId:     valobj.NewUid(),
		NotebookId: valobj.NewId(),
	}, entity.Spec{
		TTL: 2 * time.Minute,
		Env: map[string]string{"FOO": "bar"},
	})
	require.NoError(t, err)
	require.NotNil(t, sbx)
	require.NotEmpty(t, sbx.Id())

	t.Cleanup(func() {
		_ = manager.DeleteSandbox(context.Background(), sbx.Id())
	})

	t.Run("GetExisting", func(t *testing.T) {
		got, err := manager.GetSandbox(ctx, sbx.Id())
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, sbx.Id(), got.Id())
	})

	t.Run("RunEcho", func(t *testing.T) {
		exec, err := sbx.Run(ctx, entity.Command{
			Command: "echo 'hello from sandbox'",
		})
		require.NoError(t, err)
		assert.True(t, exec.Success(), "exit code = %d", exec.ExitCode)
		assert.Contains(t, string(exec.Stdout), "hello from sandbox")
	})

	t.Run("RunEnv", func(t *testing.T) {
		exec, err := sbx.Run(ctx, entity.Command{
			Command: "echo -n $FOO",
		})
		require.NoError(t, err)
		assert.True(t, exec.Success())
		assert.Equal(t, "bar", strings.TrimSpace(string(exec.Stdout)))
	})

	t.Run("RunExitCode", func(t *testing.T) {
		exec, err := sbx.Run(ctx, entity.Command{
			Command: "exit 42",
		})
		require.NoError(t, err)
		assert.Equal(t, 42, exec.ExitCode)
	})

	t.Run("WriteReadFile", func(t *testing.T) {
		data := []byte("hello from host file\n")
		err := sbx.WriteFile(ctx, "/tmp/youarehere", bytes.NewReader(data))
		require.NoError(t, err)

		content, err := sbx.ReadFile(ctx, "/tmp/youarehere")
		require.NoError(t, err)
		assert.Equal(t, data, content)

		exec, err := sbx.Run(ctx, entity.Command{
			Command: "ls -la /tmp/youarehere && cat /tmp/youarehere",
		})
		require.NoError(t, err)
		assert.True(t, exec.Success())
		assert.Contains(t, string(exec.Stdout), "hello from host file")
	})

	t.Run("Delete", func(t *testing.T) {
		err := manager.DeleteSandbox(ctx, sbx.Id())
		require.NoError(t, err)
	})
}

func TestManagerGetNotExist(t *testing.T) {
	m, err := NewManager(t.Context(), testConfig())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var manager repository.Manager = m

	sbx, err := manager.GetSandbox(ctx, "no-such-sandbox-id")
	require.Error(t, err)
	assert.Nil(t, sbx)
}
