package opensandbox

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/repository"
	pkgerr "github.com/gonotelm-lab/gonotelm/pkg/errors"

	osb "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

type Manager struct {
	lc *osb.LifecycleClient

	osbConfig osb.ConnectionConfig

	mu  sync.RWMutex
	sbs map[string]*osb.Sandbox
}

func (m *Manager) setSandbox(id string, sb *osb.Sandbox) {
	m.mu.Lock()
	m.sbs[id] = sb
	m.mu.Unlock()
}

func (m *Manager) getSandbox(id string) *osb.Sandbox {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sbs[id]
}

func (m *Manager) deleteSandbox(id string) *osb.Sandbox {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.sbs[id]
	if ok {
		delete(m.sbs, id)
		return c
	}

	return nil
}

// alpineManager 创建 alpine 镜像沙箱的 manager。
type alpineManager struct {
	*Manager
}

var _ repository.Manager = &alpineManager{}

func New(c Config) (*alpineManager, error) {
	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("opensandbox invalid endpoint %s, %w", c.Endpoint, err)
	}

	osbCfg := osb.ConnectionConfig{
		Domain:     u.Host,
		Protocol:   u.Scheme,
		APIKey:     c.ApiKey,
		HTTPClient: c.HttpClient,
	}

	lc := osb.NewLifecycleClientWithCache(
		c.Endpoint+"/"+osb.APIVersion,
		c.ApiKey,
		osb.NewEndpointCache(osb.DefaultEndpointCacheSize, osb.DefaultEndpointCacheTTL),
		osb.WithHTTPClient(c.HttpClient),
	)

	return &alpineManager{
		Manager: &Manager{
			lc:        lc,
			osbConfig: osbCfg,
			sbs:       make(map[string]*osb.Sandbox),
		},
	}, nil
}

func (m *alpineManager) CreateSandbox(ctx context.Context, spec entity.Spec) (entity.Sandbox, error) {
	var ttl *int
	if ttlSec := int(spec.TTL.Seconds()); ttlSec > 0 {
		ttl = &ttlSec
	}

	newSandbox, err := osb.CreateSandbox(ctx, m.osbConfig,
		osb.SandboxCreateOptions{
			Image: "alpine:3.23.5",
			ResourceLimits: osb.ResourceLimits{
				"cpu":    "500m",
				"memory": "32Mi",
			},
			TimeoutSeconds: ttl,
			Env:            spec.Env,
		})
	if err != nil {
		return nil, pkgerr.Wrapf(err, "opensandbox create failed")
	}

	m.setSandbox(newSandbox.ID(), newSandbox)

	return newAlpineSandbox(newSandbox), nil
}

func (m *Manager) GetSandbox(ctx context.Context, sandboxId string) (entity.Sandbox, error) {
	target := m.getSandbox(sandboxId)
	if target != nil {
		return newAlpineSandbox(target), nil
	}

	// try remote
	target, err := osb.ConnectSandbox(ctx, m.osbConfig, sandboxId)
	if err != nil {
		return nil, pkgerr.Wrapf(err, "opensandbox can not connect to %s", sandboxId)
	}

	m.setSandbox(target.ID(), target)

	return newAlpineSandbox(target), nil
}

func (m *Manager) DeleteSandbox(ctx context.Context, sandboxId string) error {
	target := m.deleteSandbox(sandboxId)
	if target != nil {
		if err := target.Kill(ctx); err != nil {
			return pkgerr.Wrapf(err, "opensandbox kill %s failed", sandboxId)
		}
	} else {
		err := m.lc.DeleteSandbox(ctx, sandboxId)
		if err != nil {
			return pkgerr.Wrapf(err, "opensandbox delete %s failed", sandboxId)
		}
	}

	return nil
}
