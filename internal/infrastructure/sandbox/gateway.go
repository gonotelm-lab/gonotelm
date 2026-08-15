package sandbox

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/sandbox/opensandbox"
	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
)

type Gateway struct {
	mu sync.RWMutex

	cfg       *ProviderConfig
	providers map[Provider]repository.Manager
}

func NewGateway(ctx context.Context, cfg *ProviderConfig) (*Gateway, error) {
	if cfg == nil {
		return nil, fmt.Errorf("sandbox config must not be nil")
	}

	gw := &Gateway{
		cfg:       cfg,
		providers: make(map[Provider]repository.Manager),
	}

	openSandbox, err := newOpenSandboxManager(cfg.OpenSandbox)
	if err != nil {
		return nil, err
	}
	gw.providers[ProviderOpenSandbox] = openSandbox

	return gw, nil
}

func (g *Gateway) GetProvider(providerType Provider) (repository.Manager, error) {
	if providerType == "" {
		return nil, fmt.Errorf("sandbox provider type must not be empty")
	}

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

	return nil, fmt.Errorf("sandbox provider %s not found", providerType)
}

func newOpenSandboxManager(cfg OpenSandboxConfig) (repository.Manager, error) {
	return opensandbox.New(opensandbox.Config{
		Endpoint:   cfg.Endpoint,
		ApiKey:     cfg.ApiKey,
		HttpClient: newHttpClient(cfg.Timeout),
	})
}

func newHttpClient(timeout time.Duration) *http.Client {
	builder := httpclient.NewBuilder(nil)
	if timeout > 0 {
		builder = builder.WithTimeout(timeout)
	}

	return builder.Build()
}
