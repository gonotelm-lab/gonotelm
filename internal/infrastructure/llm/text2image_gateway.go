package llm

import (
	"fmt"
	"sync"

	pkgt2i "github.com/gonotelm-lab/multimodal/image"
)

type Text2ImageGateway struct {
	mu sync.RWMutex

	cfg        *Text2ImageConfig
	clientOpts []pkgt2i.ClientOption
	providers  map[Text2ImageProvider]pkgt2i.Generator
}

func NewText2ImageGateway(cfg *Text2ImageConfig, opts ...pkgt2i.ClientOption) (*Text2ImageGateway, error) {
	if cfg == nil {
		return nil, fmt.Errorf("text2image config must not be nil")
	}

	gw := &Text2ImageGateway{
		cfg:        cfg,
		clientOpts: opts,
		providers:  make(map[Text2ImageProvider]pkgt2i.Generator),
	}

	defaultProvider := cfg.Type
	if defaultProvider == "" {
		defaultProvider = Text2ImageDashScope
	}

	if _, err := gw.initProvider(defaultProvider); err != nil {
		return nil, err
	}

	return gw, nil
}

func (g *Text2ImageGateway) GetProvider(providerType Text2ImageProvider) (pkgt2i.Generator, error) {
	if providerType == "" {
		return nil, fmt.Errorf("text2image provider type must not be empty")
	}
	return g.initProvider(providerType)
}

func (g *Text2ImageGateway) initProvider(providerType Text2ImageProvider) (pkgt2i.Generator, error) {
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
	provider, err := newText2ImageGenerator(providerType, &cfgCopy, g.clientOpts...)
	if err != nil {
		return nil, err
	}

	g.providers[providerType] = provider
	return provider, nil
}
