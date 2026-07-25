package rerank

import (
	"fmt"
	"sync"

	pkgrerank "github.com/gonotelm-lab/gonotelm/pkg/rerank"
)

type RerankGateway struct {
	mu sync.RWMutex

	cfg        *RerankConfig
	clientOpts []pkgrerank.ClientOption
	providers  map[RerankProvider]pkgrerank.Reranker
}

func NewRerankGateway(cfg *RerankConfig, opts ...pkgrerank.ClientOption) (*RerankGateway, error) {
	if cfg == nil {
		return nil, fmt.Errorf("rerank config must not be nil")
	}

	return &RerankGateway{
		cfg:        cfg,
		clientOpts: opts,
		providers:  make(map[RerankProvider]pkgrerank.Reranker),
	}, nil
}

func (g *RerankGateway) GetProvider(providerType RerankProvider) (pkgrerank.Reranker, error) {
	if providerType == "" {
		return nil, fmt.Errorf("rerank provider type must not be empty")
	}
	return g.initProvider(providerType)
}

func (g *RerankGateway) initProvider(providerType RerankProvider) (pkgrerank.Reranker, error) {
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
	provider, err := newReranker(providerType, &cfgCopy, g.clientOpts...)
	if err != nil {
		return nil, err
	}

	g.providers[providerType] = provider
	return provider, nil
}
