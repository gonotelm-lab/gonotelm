package sandbox

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGatewayConfig() *ProviderConfig {
	return &ProviderConfig{
		OpenSandbox: OpenSandboxConfig{
			Endpoint: "http://localhost:23080",
			ApiKey:   "123456",
		},
	}
}

func TestNewGateway(t *testing.T) {
	_, err := NewGateway(context.Background(), testGatewayConfig())
	require.NoError(t, err)
}

func TestNewGatewayNilConfig(t *testing.T) {
	_, err := NewGateway(context.Background(), nil)
	require.Error(t, err)
}

func TestNewGatewayInvalidEndpoint(t *testing.T) {
	cfg := testGatewayConfig()
	cfg.OpenSandbox.Endpoint = "://bad-endpoint"

	_, err := NewGateway(context.Background(), cfg)
	require.Error(t, err)
}

func TestGatewayGetProvider(t *testing.T) {
	gw, err := NewGateway(context.Background(), testGatewayConfig())
	require.NoError(t, err)

	manager, err := gw.GetManager(ProviderOpenSandbox)
	require.NoError(t, err)
	assert.NotNil(t, manager)

	manager2, err := gw.GetManager(ProviderOpenSandbox)
	require.NoError(t, err)
	assert.NotNil(t, manager2)
}

func TestGatewayGetLocalProvider(t *testing.T) {
	gw, err := NewGateway(context.Background(), testGatewayConfig())
	require.NoError(t, err)

	manager, err := gw.GetManager(ProviderLocal)
	require.NoError(t, err)
	assert.NotNil(t, manager)
}

func TestGatewayGetProviderNotExist(t *testing.T) {
	gw, err := NewGateway(context.Background(), testGatewayConfig())
	require.NoError(t, err)

	manager, err := gw.GetManager("no-such-provider")
	require.Error(t, err)
	assert.Nil(t, manager)
}

func TestGatewayGetProviderEmpty(t *testing.T) {
	gw, err := NewGateway(context.Background(), testGatewayConfig())
	require.NoError(t, err)

	manager, err := gw.GetManager("")
	require.Error(t, err)
	assert.Nil(t, manager)
}
