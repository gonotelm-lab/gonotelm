package embedding

import (
	"context"
	"fmt"
	"sync"

	embedcache "github.com/cloudwego/eino-ext/components/embedding/cache"
	einoembed "github.com/cloudwego/eino/components/embedding"
)

// Gateway 管理项目中的 embedding 提供商实例，并按需返回对应 embedder。
type Gateway struct {
	mu sync.RWMutex

	cfg       *Config
	cacher    embedcache.Cacher
	providers map[Type]einoembed.Embedder
}

func NewGateway(cfg *Config, cacher embedcache.Cacher) (*Gateway, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding config must not be nil")
	}

	gw := &Gateway{
		cfg:       cfg,
		cacher:    cacher,
		providers: make(map[Type]einoembed.Embedder),
	}

	defaultProvider := cfg.Type
	if defaultProvider == "" {
		defaultProvider = DashScope
	}

	if _, err := gw.initProvider(defaultProvider); err != nil {
		return nil, err
	}

	return gw, nil
}

func (g *Gateway) GetProvider(providerType Type) (einoembed.Embedder, error) {
	if providerType == "" {
		return nil, fmt.Errorf("embedding provider type must not be empty")
	}
	return g.initProvider(providerType)
}

func (g *Gateway) initProvider(providerType Type) (einoembed.Embedder, error) {
	g.mu.RLock()
	provider, ok := g.providers[providerType]
	g.mu.RUnlock()
	if ok {
		return provider, nil
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if provider, ok = g.providers[providerType]; ok {
		return provider, nil
	}

	cfgCopy := *g.cfg
	cfgCopy.Type = providerType
	provider, err := New(
		context.Background(), &cfgCopy, g.cacher,
	)
	if err != nil {
		return nil, err
	}

	g.providers[providerType] = provider
	return provider, nil
}
