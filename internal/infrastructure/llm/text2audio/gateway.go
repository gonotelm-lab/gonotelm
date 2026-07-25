package text2audio

import (
	"fmt"
	"sync"

	audios "github.com/gonotelm-lab/multimodal/audio"
)

type Text2AudioGateway struct {
	mu sync.RWMutex

	cfg        *Text2AudioConfig
	clientOpts []audios.ClientOption
	providers  map[Text2AudioProvider]audios.Generator
}

func NewText2AudioGateway(cfg *Text2AudioConfig, opts ...audios.ClientOption) (*Text2AudioGateway, error) {
	if cfg == nil {
		return nil, fmt.Errorf("text2audio config must not be nil")
	}

	return &Text2AudioGateway{
		cfg:        cfg,
		clientOpts: opts,
		providers:  make(map[Text2AudioProvider]audios.Generator),
	}, nil
}

func (g *Text2AudioGateway) GetProvider(providerType Text2AudioProvider) (audios.Generator, error) {
	if providerType == "" {
		return nil, fmt.Errorf("text2audio provider type must not be empty")
	}
	return g.initProvider(providerType)
}

func (g *Text2AudioGateway) initProvider(providerType Text2AudioProvider) (audios.Generator, error) {
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
	provider, err := newText2AudioGenerator(providerType, &cfgCopy, g.clientOpts...)
	if err != nil {
		return nil, err
	}

	g.providers[providerType] = provider
	return provider, nil
}
