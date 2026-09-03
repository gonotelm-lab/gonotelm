package embedding

import (
	"context"
	"fmt"
	"sync"

	embedcache "github.com/cloudwego/eino-ext/components/embedding/cache"
	einoembed "github.com/cloudwego/eino/components/embedding"
)

type EmbeddingGateway struct {
	mu sync.RWMutex

	cfg       *EmbeddingConfig
	cacher    embedcache.Cacher
	recorder  Recorder
	providers map[EmbeddingType]einoembed.Embedder
}

type gatewayOption struct {
	recorder Recorder
}

type GatewayOption func(o *gatewayOption)

func WithRecorder(r Recorder) GatewayOption {
	return func(o *gatewayOption) {
		o.recorder = r
	}
}

func NewEmbeddingGateway(
	cfg *EmbeddingConfig,
	cacher embedcache.Cacher,
	opts ...GatewayOption,
) (*EmbeddingGateway, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding config must not be nil")
	}

	opt := gatewayOption{}
	for _, o := range opts {
		if o != nil {
			o(&opt)
		}
	}

	gw := &EmbeddingGateway{
		cfg:       cfg,
		cacher:    wrapCacher(cacher),
		recorder:  opt.recorder,
		providers: make(map[EmbeddingType]einoembed.Embedder),
	}

	defaultProvider := cfg.Type
	if defaultProvider == "" {
		defaultProvider = EmbeddingQwen
	}

	if _, err := gw.initProvider(defaultProvider); err != nil {
		return nil, err
	}

	return gw, nil
}

func (g *EmbeddingGateway) GetProvider(providerType EmbeddingType) (einoembed.Embedder, error) {
	if providerType == "" {
		return nil, fmt.Errorf("embedding provider type must not be empty")
	}
	return g.initProvider(providerType)
}

func (g *EmbeddingGateway) initProvider(providerType EmbeddingType) (einoembed.Embedder, error) {
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
	impl, err := newEmbedder(
		context.Background(), &cfgCopy, g.cacher,
	)
	if err != nil {
		return nil, err
	}
	provider = newWrappedEmbedder(providerType, impl, g.recorder)

	g.providers[providerType] = provider
	return provider, nil
}
