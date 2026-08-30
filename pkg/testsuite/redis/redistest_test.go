package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTestRedisFromEnv_RequiresAddrs(t *testing.T) {
	t.Setenv(EnvGonotelmTestRedisAddrs, "")
	_, err := NewTestRedisFromEnv()
	require.Error(t, err)
	assert.Contains(t, err.Error(), EnvGonotelmTestRedisAddrs)
}

func TestNewTestRedisFromEnv_ParsesAddrs(t *testing.T) {
	t.Setenv(EnvGonotelmTestRedisAddrs, "127.0.0.1:6379, 127.0.0.1:6380")
	r, err := NewTestRedisFromEnv()
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, []string{"127.0.0.1:6379", "127.0.0.1:6380"}, r.addrs)
	assert.Nil(t, r.client)
}

func TestNewTestRedis_RejectsEmptyAddrs(t *testing.T) {
	_, err := NewTestRedis([]string{"", "  "})
	require.Error(t, err)
}
