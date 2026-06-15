package text2image

import (
	"fmt"
	"sync"

	pkgt2i "github.com/gonotelm-lab/multimodal/image"
)

// Gateway 管理项目中的 text2image 提供商实例，并按需返回对应 generator。
type Gateway struct {
	mu sync.RWMutex

	cfg        *Config
	clientOpts []pkgt2i.ClientOption
	providers  map[Provider]pkgt2i.Generator
}

func NewGateway(cfg *Config, opts ...pkgt2i.ClientOption) (*Gateway, error) {
	if cfg == nil {
		return nil, fmt.Errorf("text2image config must not be nil")
	}

	gw := &Gateway{
		cfg:        cfg,
		clientOpts: opts,
		providers:  make(map[Provider]pkgt2i.Generator),
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

func (g *Gateway) GetProvider(providerType Provider) (pkgt2i.Generator, error) {
	if providerType == "" {
		return nil, fmt.Errorf("text2image provider type must not be empty")
	}
	return g.initProvider(providerType)
}

func (g *Gateway) initProvider(providerType Provider) (pkgt2i.Generator, error) {
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
	provider, err := New(providerType, &cfgCopy, g.clientOpts...)
	if err != nil {
		return nil, err
	}

	g.providers[providerType] = provider
	return provider, nil
}
