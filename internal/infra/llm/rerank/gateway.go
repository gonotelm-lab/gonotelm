package rerank

import (
	"fmt"
	"sync"

	pkgrerank "github.com/gonotelm-lab/gonotelm/pkg/rerank"
)

// Gateway 管理项目中的 rerank 提供商实例，并按需返回对应 reranker。
type Gateway struct {
	mu sync.RWMutex

	cfg        *Config
	clientOpts []pkgrerank.ClientOption
	providers  map[Provider]pkgrerank.Reranker
}

func NewGateway(cfg *Config, opts ...pkgrerank.ClientOption) (*Gateway, error) {
	if cfg == nil {
		return nil, fmt.Errorf("rerank config must not be nil")
	}

	gw := &Gateway{
		cfg:        cfg,
		clientOpts: opts,
		providers:  make(map[Provider]pkgrerank.Reranker),
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

func (g *Gateway) GetProvider(providerType Provider) (pkgrerank.Reranker, error) {
	if providerType == "" {
		return nil, fmt.Errorf("rerank provider type must not be empty")
	}
	return g.initProvider(providerType)
}

func (g *Gateway) initProvider(providerType Provider) (pkgrerank.Reranker, error) {
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
