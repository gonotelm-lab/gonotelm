package cleanup

import (
	"net/http"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
)

type Config struct {
	CallbackBaseURL string
	Delay           time.Duration
	Jitter          time.Duration
}

type Deps struct {
	ScheduleClient flow.ScheduleClient

	SourceRepo    sourcerepo.Repository
	NotebookRepo  notebookrepo.Repository
	ArtifactRepo  artifactrepo.Repository
	SourceDocRepo sourcerepo.SourceDocRepository
	ObjectStore   adapter.ObjectStore
}

// Cleaner 删除兜底清理：删除接口注册检查点，事件订阅与 flow 回调执行清扫。
type Cleaner struct {
	*sweeper
	*checkpointScheduler
	*notebookRegistrar
}

func New(cfg Config, deps Deps) *Cleaner {
	sw := newSweeper(
		deps.SourceRepo,
		deps.NotebookRepo,
		deps.ArtifactRepo,
		deps.SourceDocRepo,
		deps.ObjectStore,
	)
	scheduler := newCheckpointScheduler(deps.ScheduleClient, cfg)

	return &Cleaner{
		sweeper:             sw,
		checkpointScheduler: scheduler,
		notebookRegistrar:   newNotebookRegistrar(sw, scheduler),
	}
}

// Enabled 未配置回调地址或没有 flow 客户端时不注册检查点，也不挂回调端点。
func (c *Cleaner) Enabled() bool {
	return c.enabled()
}

// VerifyCallback 校验 flow 回调的签名。
func (c *Cleaner) VerifyCallback(req *http.Request) error {
	return c.client.Verify(req)
}
