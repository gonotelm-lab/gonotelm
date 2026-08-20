package opensandbox

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/sandbox/workspace"
	pkgerr "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"

	aliosb "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

const (
	MetadataUserIdKey     = "x-meta-user-id"
	MetadataNotebookIdKey = "x-meta-notebook-id"
	MetadataServiceKey    = "x-meta-service"
	MetadataServiceValue  = "gonotelm"
)

var sandboxDefaultResourceLimit = aliosb.ResourceLimits{
	"cpu":    "500m",
	"memory": "128Mi",
}

// sandboxEntry 本地缓存的沙箱及其绑定 key
type sandboxEntry struct {
	osb *aliosb.Sandbox
	key entity.SandboxKey
}

type Manager struct {
	rootCtx   context.Context
	lifecycle *aliosb.LifecycleClient

	osbConfig aliosb.ConnectionConfig

	mu      sync.RWMutex
	entries map[string]*sandboxEntry

	c Config
}

var _ repository.Manager = &Manager{}

func NewManager(ctx context.Context, c Config) (*Manager, error) {
	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("opensandbox invalid endpoint %s, %w", c.Endpoint, err)
	}
	retryConfig := aliosb.DefaultRetryConfig()
	osbCfg := aliosb.ConnectionConfig{
		Domain:     u.Host,
		Protocol:   u.Scheme,
		APIKey:     c.ApiKey,
		HTTPClient: c.HttpClient,
		Retry:      &retryConfig,
	}

	lc := aliosb.NewLifecycleClientWithCache(
		c.Endpoint+"/"+aliosb.APIVersion,
		c.ApiKey,
		aliosb.NewEndpointCache(aliosb.DefaultEndpointCacheSize, aliosb.DefaultEndpointCacheTTL),
		aliosb.WithHTTPClient(c.HttpClient),
	)

	return &Manager{
		c:         c,
		lifecycle: lc,
		osbConfig: osbCfg,
		entries:   make(map[string]*sandboxEntry),
	}, nil
}

func (m *Manager) setOpenSandbox(id string, key entity.SandboxKey, sb *aliosb.Sandbox) {
	m.mu.Lock()
	m.entries[id] = &sandboxEntry{osb: sb, key: key}
	m.mu.Unlock()
}

func (m *Manager) getOpenSandbox(id string) *sandboxEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.entries[id]
}

func (m *Manager) deleteOpenSandbox(id string) *sandboxEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.entries[id]
	if ok {
		delete(m.entries, id)
		return old
	}

	return nil
}

func (m *Manager) CreateSandbox(ctx context.Context, key entity.SandboxKey, spec entity.Spec) (entity.Sandbox, error) {
	var ttl *int
	if ttlSec := int(spec.TTL.Seconds()); ttlSec > 0 {
		ttl = &ttlSec
	}

	openSandbox, err := aliosb.CreateSandbox(ctx, m.osbConfig,
		aliosb.SandboxCreateOptions{
			Image:          m.c.Image,
			ResourceLimits: sandboxDefaultResourceLimit,
			TimeoutSeconds: ttl,
			Env:            spec.Env,
			Metadata: map[string]string{
				MetadataUserIdKey:     key.UserId,
				MetadataNotebookIdKey: key.NotebookId.String(),
				MetadataServiceKey:    MetadataServiceValue,
			},
		})
	if err != nil {
		return nil, pkgerr.Wrapf(err, "opensandbox create failed: %s", key)
	}

	m.setOpenSandbox(openSandbox.ID(), key, openSandbox)

	if err := m.prepareSandbox(ctx, key, openSandbox); err != nil {
		// 初始化失败就认为创建失败 同时删掉已经创建的沙箱
		defer func() {
			safe.DetachGo(ctx, m.rootCtx, "opensandbox.manager.prepare.after_failure", func(ctx context.Context) {
				if e2 := m.DeleteSandbox(ctx, openSandbox.ID()); e2 != nil {
					slog.WarnContext(ctx, "opensandbox manager failed to delete sandbox after prepare failure", slog.Any("err", e2))
				}
			})
		}()

		return nil, pkgerr.Wrapf(err, "manager failed to prepare sandbox %s", openSandbox.ID())
	}

	runtime := getOpenSandboxRuntime(ctx, key, openSandbox)

	return NewCustomSandbox(openSandbox, key, runtime), nil
}

// 初始化沙箱workspace环境
func (m *Manager) prepareSandbox(ctx context.Context, key entity.SandboxKey, openSandbox *aliosb.Sandbox) error {
	// 1. mkdir -p /tmp/{userId}/{notebookId}/vendor
	vendorDir := key.WorkspaceDir() + "/vendor"
	err := openSandbox.CreateDirectory(ctx, vendorDir, 755) // 这里用十进制表示八进制
	if err != nil {
		return pkgerr.Wrapf(err, "opensandbox create directory failed: %s", key)
	}

	// 2. upload vendors（每次 Open 新句柄；读完后关闭）
	vendors := workspace.Vendors()
	uploadEntries := make([]aliosb.UploadFileEntry, 0, len(vendors))
	for fileName, file := range vendors {
		uploadEntries = append(uploadEntries, aliosb.UploadFileEntry{
			File: file,
			Options: aliosb.UploadFileOptions{
				FileName: fileName,
				Metadata: aliosb.FileMetadata{
					Path: vendorDir + "/" + fileName,
					Mode: 755,
				},
			},
		})
	}
	err = openSandbox.UploadFiles(ctx, uploadEntries)
	for _, file := range vendors {
		_ = file.Close()
	}
	if err != nil {
		return pkgerr.Wrapf(err, "opensandbox upload files failed: %s", key)
	}

	return nil
}

func (m *Manager) GetSandbox(ctx context.Context, sandboxId string) (entity.Sandbox, error) {
	target := m.getOpenSandbox(sandboxId)
	if target != nil {
		return NewCustomSandbox(target.osb, target.key, ""), nil
	}

	// try remote
	targetSb, err := aliosb.ConnectSandbox(ctx, m.osbConfig, sandboxId)
	if err != nil {
		return nil, pkgerr.Wrapf(err, "opensandbox can not connect to %s", sandboxId)
	}

	key := entity.SandboxKey{}
	runtime := ""
	if info, err := targetSb.GetInfo(ctx); err == nil {
		key = keyFromMetadata(info.Metadata)
		if info.Image != nil {
			runtime = info.Image.URI
		}
	}

	m.setOpenSandbox(targetSb.ID(), key, targetSb)

	return NewCustomSandbox(targetSb, key, runtime), nil
}

// keyFromMetadata 从沙箱元信息中还原绑定的 key
func keyFromMetadata(meta map[string]string) entity.SandboxKey {
	key := entity.SandboxKey{UserId: meta[MetadataUserIdKey]}
	if nbId, ok := meta[MetadataNotebookIdKey]; ok {
		if id, err := valobj.NewIdFromString(nbId); err == nil {
			key.NotebookId = id
		}
	}

	return key
}

func (m *Manager) DeleteSandbox(ctx context.Context, sandboxId string) error {
	target := m.deleteOpenSandbox(sandboxId)
	if target != nil {
		if err := target.osb.Kill(ctx); err != nil {
			return pkgerr.Wrapf(err, "opensandbox kill %s failed", sandboxId)
		}
	} else {
		err := m.lifecycle.DeleteSandbox(ctx, sandboxId)
		if err != nil {
			return pkgerr.Wrapf(err, "opensandbox delete %s failed", sandboxId)
		}
	}

	return nil
}

func getOpenSandboxRuntime(ctx context.Context, key entity.SandboxKey, openSandbox *aliosb.Sandbox) string {
	output, err := openSandbox.RunCommand(ctx,
		"(echo 'uname -a:' && uname -a && echo 'os-release:' && cat /etc/os-release) || true",
		nil,
	)
	if err != nil {
		slog.ErrorContext(ctx, "opensandbox run command failed",
			slog.Any("err", err), slog.String("sanbox_key", key.String()),
		)
		return "unable to run sandbox command"
	}

	if output.ExitCode == nil || *output.ExitCode != 0 {
		getCode := func() string {
			if output.ExitCode != nil {
				return strconv.Itoa(*output.ExitCode)
			}

			return "no exit code provided"
		}
		slog.ErrorContext(ctx, "opensandbox run command to get runtime exit code not 0", slog.String("exit_code", getCode()))
		return "unable to get sandbox runtime"
	}

	return output.Text()
}
