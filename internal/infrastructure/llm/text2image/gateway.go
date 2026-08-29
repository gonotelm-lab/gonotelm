package text2image

import (
	"fmt"
	"sync"

	pkgt2i "github.com/gonotelm-lab/multimodal/image"
)

type Text2ImageGateway struct {
	mu sync.RWMutex

	cfg        *Text2ImageConfig
	clientOpts []pkgt2i.ClientOption
	recorder   Recorder
	providers  map[Text2ImageProvider]pkgt2i.Generator
}

type gatewayOption struct {
	recorder   Recorder
	clientOpts []pkgt2i.ClientOption
}

type GatewayOption func(o *gatewayOption)

func WithRecorder(r Recorder) GatewayOption {
	return func(o *gatewayOption) {
		o.recorder = r
	}
}

func WithClientOptions(opts ...pkgt2i.ClientOption) GatewayOption {
	return func(o *gatewayOption) {
		o.clientOpts = append(o.clientOpts, opts...)
	}
}

func NewText2ImageGateway(cfg *Text2ImageConfig, opts ...GatewayOption) (*Text2ImageGateway, error) {
	if cfg == nil {
		return nil, fmt.Errorf("text2image config must not be nil")
	}

	opt := gatewayOption{}
	for _, o := range opts {
		if o != nil {
			o(&opt)
		}
	}

	return &Text2ImageGateway{
		cfg:        cfg,
		clientOpts: opt.clientOpts,
		recorder:   opt.recorder,
		providers:  make(map[Text2ImageProvider]pkgt2i.Generator),
	}, nil
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
	impl, err := newText2ImageGenerator(providerType, &cfgCopy, g.clientOpts...)
	if err != nil {
		return nil, err
	}
	provider = newWrappedGenerator(providerType, impl, g.recorder)

	g.providers[providerType] = provider
	return provider, nil
}
