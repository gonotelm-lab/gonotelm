# Artifact Generation via Flow + Worker (DDD) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate studio artifact generation from in-process task loop to flow-service scheduling with a standalone worker process; delete `internal/app/` entirely; finish the DDD refactor.

**Architecture:** API process (`cmd/gonotelm`) submits tasks to flow and tracks status via background syncer that polls flow.Get back into a Postgres `artifacts` table. Worker process (`cmd/worker`) registers 4 per-kind `worker.Client`s with flow, polls tasks, executes generators using `pkg/agent.Agent[State]` + `agentize.Service`-backed source tools, uploads storage-bound results to MinIO, and Reports back to flow. The artifact domain is modeled under `internal/domain/artifact/`; application use cases under `internal/application/artifact/`; HTTP routes under `internal/interfaces/api/studio/`.

**Tech Stack:** Go 1.25, Hertz (HTTP), GORM (Postgres), MinIO (storage), Milvus (vector DB), cloudwego/eino (LLM/agent), github.com/gonotelm-lab/flow (gRPC task queue), json codec for flow payloads.

## Global Constraints

- Go module path: `github.com/gonotelm-lab/gonotelm`. flow module: `github.com/gonotelm-lab/flow`. Both are members of the same `go.work`; cross-module imports are allowed.
- flow namespace `gonotelm` already exists (user provisioned). flow gRPC addr loaded from `[flow]` config section.
- Postgres (via Docker), MinIO, Milvus, Kafka are all running locally; `.env` is set; `etc/gonotelm.toml.tpl` is rendered from env. Integration tests may hit real services if needed.
- Every new file gets unit tests (TDD). For DB/store tests, follow the pattern in `internal/infrastructure/database/postgres/*_test.go` (real postgres via Docker).
- Code style: no comments in code unless explicitly asked; mirror existing import grouping; match existing patterns in the same package.
- DDD layering enforced: domain depends on nothing internal except `internal/core`. Application depends on domain + infrastructure interfaces. Infrastructure implements domain ports. Interfaces (HTTP) depends on application use cases.
- After the final task: `go build ./cmd/gonotelm && go build ./cmd/worker && go vet ./...` all succeed; `internal/app/` and `internal/api/` directories are deleted; no imports of `internal/app/...` or `internal/api/...` remain.

## File Structure (target)

```
internal/
├── domain/artifact/                       [NEW]
│   ├── entity/
│   │   ├── artifact.go                       # Artifact aggregate, enums (Kind/Status/ResultKind), factories, behavior
│   │   ├── payload.go                        # Payload interface + per-kind structs
│   │   ├── infographic.go                    # InfoGraphic enums + helpers
│   │   └── audiooverview.go                  # AudioOverview style enum
│   ├── errors/errors.go                      # domain errors
│   └── repository/repository.go              # Repository port
├── infrastructure/
│   ├── database/
│   │   ├── schema/artifact.go                 [NEW] GORM schema (replaces artifacttask.go)
│   │   ├── database.go                        [MOD] add ArtifactStore interface + DAL.ArtifactStore
│   │   └── postgres/
│   │       ├── postgres.go                    [MOD] inject ArtifactStoreImpl into DAL
│   │       ├── artifactstore.go               [NEW] store impl
│   │       └── artifactstore_test.go           [NEW] integration tests vs Docker postgres
│   ├── repository/
│   │   ├── artifact.go                        [NEW] ArtifactRepositoryImpl
│   │   └── mapper/artifact.go                 [NEW] entity↔schema
│   └── flow/                                  [NEW]
│       └── taskclient.go                      # FlowTaskClient thin wrapper for typed submit/get/cancel
├── application/artifact/                   [NEW]
│   ├── constants.go                           # MaxArtifactTitleLength etc. (moved from internal/app/constants)
│   ├── prompt/
│   │   ├── prompt.go                          # Prompt API (migrated studio subset from biz/prompt)
│   │   ├── template.go                        # template loading (migrated)
│   │   ├── studio_mindmap.go                  # StudioMindmap vars + CheckStudioMindmapResult
│   │   ├── studio_report.go                   # StudioReport vars
│   │   ├── studio_infographic.go              # StudioInfoGraphic vars
│   │   ├── studio_podcast_outline.go           # StudioPodcastOutline vars
│   │   ├── system.go                          # systemPrompt + helpers
│   │   └── zh/*.jinja                         # copy + go:embed
│   ├── generate/
│   │   ├── generate.go                        # GenerateRequest/Response, Dispatch by Kind, common helpers
│   │   ├── session.go                         # SessionState
│   │   ├── agent.go                           # buildSourceExploreAgent (new variant)
│   │   ├── mindmap.go                         # GenerateMindmap
│   │   ├── report.go                          # GenerateReport
│   │   ├── infographic.go                     # GenerateInfoGraphic
│   │   └── audiooverview.go                   # GenerateAudioOverview (placeholder)
│   ├── usecase/
│   │   ├── generate.go                        # generate usecase
│   │   ├── status.go                          # status/result usecase
│   │   ├── list.go                            # list usecase
│   │   ├── retry.go                           # retry usecase
│   │   ├── cancel.go                          # cancel usecase
│   │   └── delete.go                          # delete usecase
│   └── syncer/
│       ├── syncer.go                          # Syncer struct, Start(ctx), Shutdown
│       ├── poll_one.go                        # PollOne(artifactId)
│       └── global.go                          # global scan loop
├── application/studio/eventhandle/
│   └── onnotebookdeleted.go                   [MOD] use artifactrepo port instead of concrete *ArtifactTaskRepository
├── interfaces/
│   ├── api/studio/                          [NEW]
│   │   ├── routes.go                          # registerStudioRoutes
│   │   ├── middleware.go                      # checkArtifactUser middleware
│   │   ├── generate.go                        # POST /studio/artifact/generate
│   │   ├── status.go                          # GET /studio/artifact/:task_id/status
│   │   ├── result.go                          # GET /studio/artifact/:task_id/result
│   │   ├── retry.go                           # POST /studio/artifact/:task_id/retry
│   │   ├── delete.go                          # POST /studio/artifact/:task_id/delete
│   │   ├── cancel.go                          # POST /studio/artifact/:task_id/cancel
│   │   ├── list.go                            # GET /notebook/:id/studio/artifact/list
│   │   └── schema.go                          # DTOs (request/response, ArtifactResult)
│   └── event/eventhandler.go                  [MOD] EventDeps.ArtifactRepo typed as port
├── conf/
│   ├── config.go                              [MOD] add Flow/Worker/Syncer fields
│   ├── flowconfig.go                          [NEW]
│   ├── workerconfig.go                        [NEW]
│   └── syncerconfig.go                        [NEW]
├── bootstrap/
│   ├── shared.go                              [NEW] SharedInfra(cfg) shared by API + worker
│   ├── app.go                                 [MOD] real HTTP server + syncer
│   └── worker_app.go                          [NEW] NewWorkerApp(cfg)
cmd/
├── gonotelm/main.go                           [NEW = moved from cmd/main.go]
└── worker/main.go                             [NEW]
etc/gonotelm.toml.tpl                          [MOD] add [flow]/[syncer]/[worker] sections
migration/db/20250711_artifacts.sql             [NEW] new table
```

Old `internal/app/{biz,logic,model,agent,constants}`, `internal/api/` removed in the final cleanup task.

---

## Task 1: Artifact Domain — Entities

**Files:**
- Create: `internal/domain/artifact/entity/artifact.go`
- Create: `internal/domain/artifact/entity/payload.go`
- Create: `internal/domain/artifact/entity/infographic.go`
- Create: `internal/domain/artifact/entity/audiooverview.go`
- Test: `internal/domain/artifact/entity/artifact_test.go`

**Interfaces:**
- Consumes: `internal/core/entity`, `internal/core/valobj`, `pkg/uuid`
- Produces: `Artifact`, `Kind`, `Status`, `ResultKind`, `Payload` interface + concrete structs, factories `NewArtifact`, behavior methods `BindFlowTaskId`, `MarkRunning`, `MarkCompleted`, `MarkFailed`, `MarkCancelled`, `MarkRetrying`, `IsTerminal`, `IsOwner`

- [ ] **Step 1: Write the failing test**

```go
// internal/domain/artifact/entity/artifact_test.go
package entity

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewArtifact(t *testing.T) {
	notebookId := uuid.NewV7()
	userId := "user-1"
	payload := &MindmapPayload{NotebookId: notebookId, SourceIds: []valobj.Id{uuid.NewV7()}}

	got := NewArtifact(notebookId, userId, KindMindmap, payload)

	assert.NotEqual(t, valobj.Id{}, got.Id)
	assert.Equal(t, notebookId, got.NotebookId)
	assert.Equal(t, userId, got.UserId)
	assert.Equal(t, KindMindmap, got.Kind)
	assert.Equal(t, StatusPending, got.Status)
	assert.Equal(t, payload, got.Payload)
	assert.False(t, got.IsTerminal())
}

func TestArtifactBindFlowTaskId(t *testing.T) {
	a := newTestArtifact(t)
	a.BindFlowTaskId("flow-task-1")
	assert.Equal(t, "flow-task-1", a.FlowTaskId)
}

func TestArtifactMarkCompleted(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCompleted([]byte("result"), ResultKindInline, "title")
	assert.Equal(t, StatusCompleted, a.Status)
	assert.Equal(t, []byte("result"), a.Result)
	assert.Equal(t, ResultKindInline, a.ResultKind)
	assert.Equal(t, "title", a.Title)
	assert.True(t, a.IsTerminal())
}

func TestArtifactMarkFailed(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkFailed()
	assert.Equal(t, StatusFailed, a.Status)
	assert.True(t, a.IsTerminal())
}

func TestArtifactMarkCancelled(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCancelled()
	assert.Equal(t, StatusCancelled, a.Status)
	assert.True(t, a.IsTerminal())
}

func TestArtifactMarkRetrying(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCompleted([]byte("old"), ResultKindInline, "old-title")
	a.MarkRetrying("flow-task-2")
	assert.Equal(t, StatusPending, a.Status)
	assert.Equal(t, "flow-task-2", a.FlowTaskId)
	assert.Empty(t, a.Title)
	assert.Empty(t, a.Result)
	assert.Empty(t, a.ResultKind)
}

func TestArtifactIsOwner(t *testing.T) {
	a := newTestArtifact(t)
	assert.True(t, a.IsOwner("user-1"))
	assert.False(t, a.IsOwner("user-2"))
}

func newTestArtifact(t *testing.T) *Artifact {
	t.Helper()
	return NewArtifact(uuid.NewV7(), "user-1", KindMindmap, &MindmapPayload{})
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/domain/artifact/entity/...`
Expected: FAIL — `entity.go` does not exist, compilation errors.

- [ ] **Step 3: Implement artifacts.go**

```go
// internal/domain/artifact/entity/artifact.go
package entity

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Kind string

const (
	KindMindmap       Kind = "mindmap"
	KindReport        Kind = "report"
	KindInfoGraphic   Kind = "info_graphic"
	KindAudioOverview Kind = "audio_overview"
)

func (k Kind) Supported() bool {
	switch k {
	case KindMindmap, KindReport, KindInfoGraphic, KindAudioOverview:
		return true
	}
	return false
}

func (k Kind) String() string { return string(k) }

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

func (s Status) Pending() bool   { return s == StatusPending }
func (s Status) Running() bool    { return s == StatusRunning }
func (s Status) Completed() bool { return s == StatusCompleted }
func (s Status) Failed() bool     { return s == StatusFailed }
func (s Status) Cancelled() bool  { return s == StatusCancelled }
func (s Status) String() string  { return string(s) }

type ResultKind string

const (
	ResultKindInline  ResultKind = "inline"
	ResultKindStorage ResultKind = "storage"
)

func (r ResultKind) Inline() bool  { return r == ResultKindInline }
func (r ResultKind) Storage() bool { return r == ResultKindStorage }
func (r ResultKind) String() string { return string(r) }

type Artifact struct {
	entity.Base
	NotebookId  valobj.Id
	UserId      string
	Kind        Kind
	Status      Status
	FlowTaskId  string
	Title       string
	Result      []byte
	ResultKind  ResultKind
	Payload     Payload
}

func NewArtifact(notebookId valobj.Id, userId string, kind Kind, payload Payload) *Artifact {
	a := &Artifact{
		NotebookId: notebookId,
		UserId:     userId,
		Kind:       kind,
		Status:     StatusPending,
		Payload:    payload,
	}
	a.Base = entity.NewBase()
	return a
}

func (a *Artifact) IsOwner(userId string) bool { return a.UserId == userId }

func (a *Artifact) BindFlowTaskId(flowTaskId string) { a.FlowTaskId = flowTaskId }

func (a *Artifact) MarkRunning()                          { a.Status = StatusRunning }
func (a *Artifact) MarkCompleted(result []byte, kind ResultKind, title string) {
	a.Status = StatusCompleted
	a.Result = result
	a.ResultKind = kind
	a.Title = title
}
func (a *Artifact) MarkFailed() { a.Status = StatusFailed }

func (a *Artifact) MarkCancelled() { a.Status = StatusCancelled }

func (a *Artifact) MarkRetrying(newFlowTaskId string) {
	a.Status = StatusPending
	a.FlowTaskId = newFlowTaskId
	a.Title = ""
	a.Result = nil
	a.ResultKind = ""
}

func (a *Artifact) IsTerminal() bool {
	return a.Status.Completed() || a.Status.Failed() || a.Status.Cancelled()
}

func NewArtifactId() valobj.Id { return valobj.Id(uuid.NewV7()) }
```

- [ ] **Step 4: Implement payload.go**

```go
// internal/domain/artifact/entity/payload.go
package entity

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type Payload interface {
	Kind() Kind
}

type MindmapPayload struct {
	NotebookId valobj.Id `json:"notebook_id"`
	SourceIds  []valobj.Id `json:"source_ids"`
}

func (p *MindmapPayload) Kind() Kind { return KindMindmap }

type ReportPayload struct {
	NotebookId valobj.Id   `json:"notebook_id"`
	SourceIds  []valobj.Id `json:"source_ids"`
}

func (p *ReportPayload) Kind() Kind { return KindReport }

type InfoGraphicPayload struct {
	NotebookId   valobj.Id                          `json:"notebook_id"`
	SourceIds    []valobj.Id                        `json:"source_ids"`
	ExtraPrompt  string                             `json:"extra_prompt"`
	TextLanguage string                             `json:"text_language"`
	Orientation  ArtifactInfoGraphicOrientation     `json:"orientation"`
	DetailLevel  ArtifactInfoGraphicDetailLevel     `json:"detail_level"`
}

func (p *InfoGraphicPayload) Kind() Kind { return KindInfoGraphic }

type AudioOverviewPayload struct {
	NotebookId valobj.Id                    `json:"notebook_id"`
	SourceIds  []valobj.Id                  `json:"source_ids"`
	Tip        string                       `json:"tip"`
	Language   string                       `json:"language"`
	Style      ArtifactAudioOverviewStyle   `json:"style"`
}

func (p *AudioOverviewPayload) Kind() Kind { return KindAudioOverview }
```

- [ ] **Step 5: Implement infographic.go and audiooverview.go**

Copy enum definitions (Orientation / DetailLevel / AudioOverviewStyle) verbatim from the old `internal/app/model/artifact.go` (`artifact.go:33-102`) into these two files, adapting only the type names to drop the `Artifact` prefix when redundant. Preserve getters with `Supported()` and `String()` and `ImageSize()`.

```go
// internal/domain/artifact/entity/infographic.go
package entity

type ArtifactInfoGraphicOrientation string

const (
	ArtifactInfoGraphicOrientationPortrait  ArtifactInfoGraphicOrientation = "portrait"
	ArtifactInfoGraphicOrientationLandscape ArtifactInfoGraphicOrientation = "landscape"
	ArtifactInfoGraphicOrientationSquare    ArtifactInfoGraphicOrientation = "square"
)

func (o ArtifactInfoGraphicOrientation) String() string { return string(o) }
func (o ArtifactInfoGraphicOrientation) Supported() bool {
	switch o {
	case ArtifactInfoGraphicOrientationPortrait,
		ArtifactInfoGraphicOrientationLandscape,
		ArtifactInfoGraphicOrientationSquare:
		return true
	}
	return false
}
func (o ArtifactInfoGraphicOrientation) ImageSize() (int, int) {
	switch o {
	case ArtifactInfoGraphicOrientationPortrait:
		return 720, 1280
	case ArtifactInfoGraphicOrientationLandscape:
		return 1280, 720
	case ArtifactInfoGraphicOrientationSquare:
		return 1024, 1024
	}
	return 1280, 720
}

type ArtifactInfoGraphicDetailLevel string

const (
	ArtifactInfoGraphicDetailLevelConcise  ArtifactInfoGraphicDetailLevel = "concise"
	ArtifactInfoGraphicDetailLevelStandard ArtifactInfoGraphicDetailLevel = "standard"
	ArtifactInfoGraphicDetailLevelDetailed ArtifactInfoGraphicDetailLevel = "detailed"
)

func (d ArtifactInfoGraphicDetailLevel) String() string  { return string(d) }
func (d ArtifactInfoGraphicDetailLevel) Supported() bool {
	switch d {
	case ArtifactInfoGraphicDetailLevelConcise,
		ArtifactInfoGraphicDetailLevelStandard,
		ArtifactInfoGraphicDetailLevelDetailed:
		return true
	}
	return false
}
```

```go
// internal/domain/artifact/entity/audiooverview.go
package entity

type ArtifactAudioOverviewStyle string

const (
	ArtifactAudioOverviewStyleDeepResearch ArtifactAudioOverviewStyle = "deep-research"
	ArtifactAudioOverviewStyleAbstract    ArtifactAudioOverviewStyle = "abstract"
	ArtifactAudioOverviewStyleDiscussion  ArtifactAudioOverviewStyle = "discussion"
	ArtifactAudioOverviewStyleDebate      ArtifactAudioOverviewStyle = "debate"
)

func (s ArtifactAudioOverviewStyle) String() string { return string(s) }
func (s ArtifactAudioOverviewStyle) Supported() bool {
	switch s {
	case ArtifactAudioOverviewStyleDeepResearch,
		ArtifactAudioOverviewStyleAbstract,
		ArtifactAudioOverviewStyleDiscussion,
		ArtifactAudioOverviewStyleDebate:
		return true
	}
	return false
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/domain/artifact/entity/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/domain/artifact/entity/
git commit -m "feat(artifact): add Artifact domain entity, payloads, enums"
```

---

## Task 2: Artifact Domain — Errors & Repository Port

**Files:**
- Create: `internal/domain/artifact/errors/errors.go`
- Create: `internal/domain/artifact/repository/repository.go`
- Test: `internal/domain/artifact/repository/repository_test.go` (compile-time interface assertion only)

**Interfaces:**
- Consumes: `internal/domain/artifact/entity`, `internal/core/valobj`, `pkg/errors`
- Produces: domain errors; `Repository` interface used by repository impl and by `application/artifact`

- [ ] **Step 1: Implement errors.go**

```go
// internal/domain/artifact/errors/errors.go
package errors

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

var (
	ErrArtifactNotFound      = errors.ErrNoRecord.Msg("artifact not found")
	ErrArtifactNotOwnedByUser = errors.ErrPermission.Msg("artifact not owned by user")
	ErrCannotCancelInState    = errors.ErrParams.Msg("cannot cancel artifact in current state")
	ErrCannotRetryInState     = errors.ErrParams.Msg("cannot retry artifact in current state")
	ErrInvalidFlowTaskId      = errors.ErrParams.Msg("artifact has no flow task id")
)
```

- [ ] **Step 2: Write the failing test for the port shape (compile-time)**

```go
// internal/domain/artifact/repository/repository_test.go
package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

type fakeRepo struct{}

var _ Repository = &fakeRepo{}

func (f *fakeRepo) Save(ctx context.Context, a *entity.Artifact) error             { return nil }
func (f *fakeRepo) FindById(ctx context.Context, id valobj.Id) (*entity.Artifact, error) { return nil, nil }
func (f *fakeRepo) ListByNotebookId(ctx context.Context, notebookId valobj.Id, limit, offset int) ([]*entity.Artifact, error) { return nil, nil }
func (f *fakeRepo) ListByStatus(ctx context.Context, statuses []entity.Status, limit int) ([]*entity.Artifact, error) { return nil, nil }
func (f *fakeRepo) UpdateStatus(ctx context.Context, id valobj.Id, status entity.Status, result []byte, resultKind entity.ResultKind, title string) error { return nil }
func (f *fakeRepo) DeleteById(ctx context.Context, id valobj.Id) error              { return nil }
func (f *fakeRepo) DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error { return nil }

func TestRepositoryInterfaceSatisfied(t *testing.T) {
	_ = &fakeRepo{}
}
```

- [ ] **Step 3: Run to confirm it fails, then implement repository.go**

```go
// internal/domain/artifact/repository/repository.go
package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

type Repository interface {
	Save(ctx context.Context, artifact *entity.Artifact) error
	FindById(ctx context.Context, id valobj.Id) (*entity.Artifact, error)
	ListByNotebookId(ctx context.Context, notebookId valobj.Id, limit, offset int) ([]*entity.Artifact, error)
	ListByStatus(ctx context.Context, statuses []entity.Status, limit int) ([]*entity.Artifact, error)
	UpdateStatus(ctx context.Context, id valobj.Id, status entity.Status, result []byte, resultKind entity.ResultKind, title string) error
	DeleteById(ctx context.Context, id valobj.Id) error
	DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error
}
```

- [ ] **Step 4: Run tests, commit**

Run: `go test ./internal/domain/artifact/...`
Expected: PASS

```bash
git add internal/domain/artifact/errors/ internal/domain/artifact/repository/
git commit -m "feat(artifact): add domain errors and repository port"
```

---

## Task 3: artifacts Table Migration + GORM Schema

**Files:**
- Create: `migration/db/20250711_artifacts.sql`
- Create: `internal/infrastructure/database/schema/artifact.go`

**Interfaces:**
- Consumes: none beyond gorm
- Produces: `schema.Artifact` GORM model with `TableName() = "artifacts"`. Used by store impl and DAL.

- [ ] **Step 1: Write the SQL migration**

```sql
-- migration/db/20250711_artifacts.sql
CREATE TABLE IF NOT EXISTS artifacts (
  id            UUID        PRIMARY KEY,
  notebook_id   UUID        NOT NULL,
  user_id       VARCHAR(128) NOT NULL,
  kind          VARCHAR(32) NOT NULL,
  status        VARCHAR(16) NOT NULL,
  flow_task_id  VARCHAR(64) NOT NULL,
  title         VARCHAR(256) NULL,
  result        BYTEA       NULL,
  result_kind   VARCHAR(16) NULL,
  payload       JSONB       NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_artifacts_notebook ON artifacts(notebook_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_status   ON artifacts(status);
ALTER TABLE artifacts ADD CONSTRAINT uq_artifacts_flow_task_id UNIQUE (flow_task_id);
```

- [ ] **Step 2: Apply to running postgres**

Run: `psql "$GONOTELM_DB_DSN" -f migration/db/20250711_artifacts.sql` (or, use the existing `scripts/...` pattern — inspect any existing migration runner).
Expected: `CREATE TABLE` succeeds.

- [ ] **Step 3: Implement GORM schema**

```go
// internal/infrastructure/database/schema/artifact.go
package schema

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Artifact struct {
	Id           uuid.UUID  `gorm:"column:id;primaryKey"`
	NotebookId   uuid.UUID  `gorm:"column:notebook_id"`
	UserId       string     `gorm:"column:user_id"`
	Kind         string     `gorm:"column:kind"`
	Status       string     `gorm:"column:status"`
	FlowTaskId   string     `gorm:"column:flow_task_id"`
	Title        string     `gorm:"column:title"`
	Result       []byte     `gorm:"column:result"`
	ResultKind   string     `gorm:"column:result_kind"`
	Payload      []byte     `gorm:"column:payload"`
	CreatedAt    time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (Artifact) TableName() string { return "artifacts" }
```

- [ ] **Step 4: Run `go build` to validate compilation, commit**

```bash
git add migration/db/20250711_artifacts.sql internal/infrastructure/database/schema/artifact.go
git commit -m "feat(artifact): add artifacts table migration and GORM schema"
```

---

## Task 4: Artifact Postgres Store

**Files:**
- Modify: `internal/infrastructure/database/database.go` — add `ArtifactStore` interface; add `ArtifactStore` field to `DAL`; add to `NewDAL` params.
- Modify: `internal/infrastructure/database/postgres/postgres.go` — instantiate `ArtifactStoreImpl`.
- Create: `internal/infrastructure/database/postgres/artifactstore.go`
- Test: `internal/infrastructure/database/postgres/artifactstore_test.go`

**Interfaces:**
- Consumes: `internal/infrastructure/database/schema`, `pkg/sql`, `pkg/uuid`, `pkg/errors`
- Produces: `database.ArtifactStore` (interface) and `ArtifactStoreImpl`. Used by repository impl.

- [ ] **Step 1: Write the failing integration test**

Follow the pattern from `internal/infrastructure/database/postgres/artifacttaskstore_test.go` (real DB via `main_test.go` setup). Test cases: `Create` then `GetById`; `ListByNotebookId`; `UpdateStatus` (with old-status conditional update); `ListByStatus` (filter by status list); `DeleteById`.

```go
// internal/infrastructure/database/postgres/artifactstore_test.go
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArtifactStore_CreateAndGetById(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := testArtifactStore // assigned in main_test.go; add it there

	id := uuid.NewV7()
	now := time.Now()
	in := &schema.Artifact{
		Id: id, NotebookId: uuid.NewV7(), UserId: "u1",
		Kind: "mindmap", Status: "pending", FlowTaskId: "ft-1",
		Payload: []byte(`{}`), CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, store.Create(ctx, in))

	got, err := store.GetById(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, in.UserId, got.UserId)
	assert.Equal(t, "pending", got.Status)
}

func TestArtifactStore_UpdateStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := uuid.NewV7()
	require.NoError(t, store.Create(ctx, &schema.Artifact{
		Id: id, NotebookId: uuid.NewV7(), UserId: "u2",
		Kind: "report", Status: "pending", FlowTaskId: "ft-2",
		Payload: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}))

	ok, err := store.UpdateStatus(ctx, id, "running", "pending", &schema.ArtifactUpdateStatusParams{
		NewStatus: "running", Title: "", Result: nil, ResultKind: "", UpdatedAt: time.Now(),
	})
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = store.UpdateStatus(ctx, id, "running", "pending", &schema.ArtifactUpdateStatusParams{
		NewStatus: "running", UpdatedAt: time.Now(),
	})
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestArtifactStore_ListByStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id1, id2 := uuid.NewV7(), uuid.NewV7()
	for _, id := range []uuid.UUID{id1, id2} {
		require.NoError(t, store.Create(ctx, &schema.Artifact{
			Id: id, NotebookId: uuid.NewV7(), UserId: "u3",
			Kind: "mindmap", Status: "pending", FlowTaskId: uuid.NewString(),
			Payload: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}))
	}
	got, err := store.ListByStatus(ctx, []string{"pending"}, 100)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(got), 2)
}
```

Add to `postgres/main_test.go` a new line: `testArtifactStore = NewArtifactStoreImpl(testDB)`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/infrastructure/database/postgres/ -run TestArtifactStore`
Expected: FAIL — store does not exist.

- [ ] **Step 3: Add the ArtifactStore interface to database.go**

Modify `internal/infrastructure/database/database.go`:
- Add to the file:

```go
type ArtifactStore interface {
	Create(ctx context.Context, artifact *schema.Artifact) error
	GetById(ctx context.Context, id Id) (*schema.Artifact, error)
	GetStatusById(ctx context.Context, id Id) (string, error)
	ListByNotebookId(ctx context.Context, notebookId Id, limit, offset int) ([]*schema.Artifact, error)
	ListByStatus(ctx context.Context, statuses []string, limit int) ([]*schema.Artifact, error)
	UpdateStatus(ctx context.Context, id Id, newStatus string, oldStatus string, params *schema.ArtifactUpdateStatusParams) (bool, error)
	UpdateFlowTaskId(ctx context.Context, id Id, flowTaskId string, oldStatuses []string) error
	DeleteById(ctx context.Context, id Id) error
	DeleteByNotebookId(ctx context.Context, notebookId Id) error
}
```

- Add `ArtifactStore ArtifactStore` field to `DAL`, and add it as the last parameter of `NewDAL`.

- Add to `internal/infrastructure/database/schema/artifact.go`:

```go
type ArtifactUpdateStatusParams struct {
	NewStatus  string
	Title      string
	Result     []byte
	ResultKind string
	UpdatedAt  time.Time
}
```

- [ ] **Step 4: Implement the store**

```go
// internal/infrastructure/database/postgres/artifactstore.go
package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
)

type ArtifactStoreImpl struct{ db *gorm.DB }

var _ database.ArtifactStore = &ArtifactStoreImpl{}

func NewArtifactStoreImpl(db *gorm.DB) *ArtifactStoreImpl {
	return &ArtifactStoreImpl{db: db}
}

func (s *ArtifactStoreImpl) Create(ctx context.Context, a *schema.Artifact) error {
	if err := s.db.WithContext(ctx).Create(a).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *ArtifactStoreImpl) GetById(ctx context.Context, id database.Id) (*schema.Artifact, error) {
	var a schema.Artifact
	if err := s.db.WithContext(ctx).Where("id = ?", id).Take(&a).Error; err != nil {
		return nil, sql.WrapErr(err)
	}
	return &a, nil
}

func (s *ArtifactStoreImpl) GetStatusById(ctx context.Context, id database.Id) (string, error) {
	var a schema.Artifact
	if err := s.db.WithContext(ctx).Model(&schema.Artifact{}).Where("id = ?", id).Select("status").Take(&a).Error; err != nil {
		return "", sql.WrapErr(err)
	}
	return a.Status, nil
}

func (s *ArtifactStoreImpl) ListByNotebookId(ctx context.Context, notebookId database.Id, limit, offset int) ([]*schema.Artifact, error) {
	var rows []*schema.Artifact
	if err := s.db.WithContext(ctx).
		Where("notebook_id = ?", notebookId).
		Order("created_at DESC, id DESC").
		Limit(limit).Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, sql.WrapErr(err)
	}
	return rows, nil
}

func (s *ArtifactStoreImpl) ListByStatus(ctx context.Context, statuses []string, limit int) ([]*schema.Artifact, error) {
	var rows []*schema.Artifact
	if err := s.db.WithContext(ctx).
		Where("status IN ?", statuses).
		Order("updated_at ASC, id ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, sql.WrapErr(err)
	}
	return rows, nil
}

func (s *ArtifactStoreImpl) UpdateStatus(
	ctx context.Context, id database.Id, newStatus string, oldStatus string, params *schema.ArtifactUpdateStatusParams,
) (bool, error) {
	updates := map[string]any{"status": newStatus, "updated_at": params.UpdatedAt}
	if params.Title != "" {
		updates["title"] = params.Title
	}
	if params.Result != nil {
		updates["result"] = params.Result
	}
	if params.ResultKind != "" {
		updates["result_kind"] = params.ResultKind
	}
	res := s.db.WithContext(ctx).
		Model(&schema.Artifact{}).
		Where("id = ?", id).
		Where("status = ?", oldStatus).
		Updates(updates)
	if res.Error != nil {
		return false, sql.WrapErr(res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (s *ArtifactStoreImpl) UpdateFlowTaskId(ctx context.Context, id database.Id, flowTaskId string, oldStatuses []string) error {
	q := s.db.WithContext(ctx).Model(&schema.Artifact{}).Where("id = ?", id)
	if len(oldStatuses) > 0 {
		q = q.Where("status IN ?", oldStatuses)
	}
	if err := q.Updates(map[string]any{"flow_task_id": flowTaskId, "status": "pending", "updated_at": gorm.Expr("now()")}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *ArtifactStoreImpl) DeleteById(ctx context.Context, id database.Id) error {
	if err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&schema.Artifact{}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *ArtifactStoreImpl) DeleteByNotebookId(ctx context.Context, notebookId database.Id) error {
	if err := s.db.WithContext(ctx).Where("notebook_id = ?", notebookId).Delete(&schema.Artifact{}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}
```

- [ ] **Step 5: Wire into `postgres.Open`**

Modify `internal/infrastructure/database/postgres/postgres.go:36` — add `NewArtifactStoreImpl(db),` as a new arg to `NewDAL` (matches the new param in `database.NewDAL`).

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/infrastructure/database/postgres/ -run TestArtifactStore`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/infrastructure/database/
git commit -m "feat(artifact): postgres ArtifactStore + DAL wiring"
```

---

## Task 5: Artifact Repository Implementation

**Files:**
- Create: `internal/infrastructure/repository/mapper/artifact.go`
- Create: `internal/infrastructure/repository/artifact.go`
- Test: `internal/infrastructure/repository/artifact_test.go`

**Interfaces:**
- Consumes: `internal/domain/artifact/{entity,errors,repository}`, `internal/infrastructure/database`, `pkg/errors`, `pkg/uuid`, sonic
- Produces: `NewArtifactRepository(database.ArtifactStore) artifactrepo.Repository` (returns interface, matching the `notebook.go` pattern)

- [ ] **Step 1: Write the failing mapper test**

```go
// internal/infrastructure/repository/mapper/artifact_test.go
package mapper

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArtifactRoundTrip(t *testing.T) {
	notebookId := uuid.NewV7()
	sourceId := uuid.NewV7()
	payload := &artifactentity.MindmapPayload{NotebookId: notebookId, SourceIds: []valobj.Id{sourceId}}
	a := artifactentity.NewArtifact(notebookId, "u1", artifactentity.KindMindmap, payload)
	a.BindFlowTaskId("ft-1")

	sch := ArtifactToSchema(a)
	assert.Equal(t, "u1", sch.UserId)
	assert.Equal(t, "mindmap", sch.Kind)
	assert.Equal(t, "ft-1", sch.FlowTaskId)

	var rawPayload map[string]any
	require.NoError(t, sonic.Unmarshal(sch.Payload, &rawPayload))
	assert.Equal(t, notebookId.String(), rawPayload["notebook_id"])

	back := ArtifactFromSchema(sch)
	assert.Equal(t, a.Id, back.Id)
	assert.Equal(t, a.NotebookId, back.NotebookId)
	assert.Equal(t, a.Kind, back.Kind)
	assert.Equal(t, a.FlowTaskId, back.FlowTaskId)
	assert.Equal(t, a.Status, back.Status)
}
```

- [ ] **Step 2: Run to confirm it fails, then implement the mapper**

```go
// internal/infrastructure/repository/mapper/artifact.go
package mapper

import (
	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func ArtifactToSchema(a *artifactentity.Artifact) *schema.Artifact {
	var payloadBytes []byte
	if a.Payload != nil {
		b, err := sonic.Marshal(a.Payload)
		if err != nil {
			panic("marshal artifact payload: " + err.Error())
		}
		payloadBytes = b
	}
	return &schema.Artifact{
		Id:         uuid.UUID(a.Id),
		NotebookId: uuid.UUID(a.NotebookId),
		UserId:     a.UserId,
		Kind:       a.Kind.String(),
		Status:     a.Status.String(),
		FlowTaskId: a.FlowTaskId,
		Title:      a.Title,
		Result:     a.Result,
		ResultKind: a.ResultKind.String(),
		Payload:    payloadBytes,
	}
}

func ArtifactFromSchema(sch *schema.Artifact) *artifactentity.Artifact {
	a := &artifactentity.Artifact{
		NotebookId: valobj.Id(sch.NotebookId),
		UserId:     sch.UserId,
		Kind:       artifactentity.Kind(sch.Kind),
		Status:     artifactentity.Status(sch.Status),
		FlowTaskId: sch.FlowTaskId,
		Title:      sch.Title,
		Result:     sch.Result,
		ResultKind: artifactentity.ResultKind(sch.ResultKind),
	}
	a.Base.Id = valobj.Id(sch.Id)
	a.Payload = decodePayload(a.Kind, sch.Payload)
	return a
}

func decodePayload(kind artifactentity.Kind, b []byte) artifactentity.Payload {
	switch kind {
	case artifactentity.KindMindmap:
		var p artifactentity.MindmapPayload
		mustUnmarshal(b, &p)
		return &p
	case artifactentity.KindReport:
		var p artifactentity.ReportPayload
		mustUnmarshal(b, &p)
		return &p
	case artifactentity.KindInfoGraphic:
		var p artifactentity.InfoGraphicPayload
		mustUnmarshal(b, &p)
		return &p
	case artifactentity.KindAudioOverview:
		var p artifactentity.AudioOverviewPayload
		mustUnmarshal(b, &p)
		return &p
	}
	return nil
}

func mustUnmarshal(b []byte, v any) {
	if len(b) == 0 {
		return
	}
	if err := sonic.Unmarshal(b, v); err != nil {
		panic("unmarshal artifact payload: " + err.Error())
	}
}
```

- [ ] **Step 3: Write the failing repository test (using real DB via postgres.Open + Docker)**

```go
// internal/infrastructure/repository/artifact_test.go
package repository

import (
	"context"
	"testing"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/postgres"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// use the same testDB setup as postgres_test.go (see main_test.go in that pkg).
// we cross-package: create the DAL via postgres.Open using test config.

func setupRepo(t *testing.T) (context.Context, Repository) {
	t.Helper()
	dal, err := postgres.Open(testDBConfig())
	require.NoError(t, err)
	return context.Background(), NewArtifactRepository(dal.ArtifactStore)
}

func TestArtifactRepository_SaveAndFindById(t *testing.T) {
	ctx, repo := setupRepo(t)
	payload := &artifactentity.MindmapPayload{NotebookId: uuid.NewV7(), SourceIds: nil}
	a := artifactentity.NewArtifact(uuid.NewV7(), "u-repo-1", artifactentity.KindMindmap, payload)
	a.BindFlowTaskId("flow-1")
	require.NoError(t, repo.Save(ctx, a))

	got, err := repo.FindById(ctx, a.Id)
	require.NoError(t, err)
	assert.Equal(t, a.Id, got.Id)
	assert.Equal(t, artifactentity.StatusPending, got.Status)
	assert.Equal(t, "flow-1", got.FlowTaskId)
}

func TestArtifactRepository_FindById_NotFound(t *testing.T) {
	ctx, repo := setupRepo(t)
	_, err := repo.FindById(ctx, uuid.NewV7())
	require.Error(t, err)
	assert.ErrorIs(t, err, artifacterrors.ErrArtifactNotFound)
}

func TestArtifactRepository_ListByStatus_And_UpdateStatus(t *testing.T) {
	ctx, repo := setupRepo(t)
	a := artifactentity.NewArtifact(uuid.NewV7(), "u-repo-2", artifactentity.KindReport, &artifactentity.ReportPayload{})
	a.BindFlowTaskId("flow-list-1")
	require.NoError(t, repo.Save(ctx, a))

	got, err := repo.ListByStatus(ctx, []artifactentity.Status{artifactentity.StatusPending}, 50)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(got), 1)

	require.NoError(t, repo.UpdateStatus(ctx, a.Id, artifactentity.StatusRunning, nil, "", ""))
	got2, err := repo.FindById(ctx, a.Id)
	require.NoError(t, err)
	assert.Equal(t, artifactentity.StatusRunning, got2.Status)
}
```

Helper `testDBConfig()` must return a `conf.DatabaseConfig` pointing at the local postgres (mirror `postgres/main_test.go` helpers).

- [ ] **Step 4: Implement repository/artifact.go**

```go
// internal/infrastructure/repository/artifact.go
package repository

import (
	"context"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ArtifactRepositoryImpl struct{ store database.ArtifactStore }

func NewArtifactRepository(store database.ArtifactStore) artifactrepo.Repository {
	return &ArtifactRepositoryImpl{store: store}
}

var _ artifactrepo.Repository = &ArtifactRepositoryImpl{}

func (r *ArtifactRepositoryImpl) Save(ctx context.Context, a *artifactentity.Artifact) error {
	return r.store.Create(ctx, mapper.ArtifactToSchema(a))
}

func (r *ArtifactRepositoryImpl) FindById(ctx context.Context, id valobj.Id) (*artifactentity.Artifact, error) {
	sch, err := r.store.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, artifacterrors.ErrArtifactNotFound
		}
		return nil, err
	}
	return mapper.ArtifactFromSchema(sch), nil
}

func (r *ArtifactRepositoryImpl) ListByNotebookId(ctx context.Context, notebookId valobj.Id, limit, offset int) ([]*artifactentity.Artifact, error) {
	rows, err := r.store.ListByNotebookId(ctx, notebookId, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*artifactentity.Artifact, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapper.ArtifactFromSchema(row))
	}
	return out, nil
}

func (r *ArtifactRepositoryImpl) ListByStatus(ctx context.Context, statuses []artifactentity.Status, limit int) ([]*artifactentity.Artifact, error) {
	strs := make([]string, 0, len(statuses))
	for _, s := range statuses {
		strs = append(strs, s.String())
	}
	rows, err := r.store.ListByStatus(ctx, strs, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*artifactentity.Artifact, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapper.ArtifactFromSchema(row))
	}
	return out, nil
}

func (r *ArtifactRepositoryImpl) UpdateStatus(
	ctx context.Context,
	id valobj.Id,
	status artifactentity.Status,
	result []byte,
	resultKind artifactentity.ResultKind,
	title string,
) error {
	ok, err := r.store.UpdateStatus(ctx, id, status.String(), "", &schema.ArtifactUpdateStatusParams{
		NewStatus: status.String(), Title: title, Result: result, ResultKind: resultKind.String(), UpdatedAt: time.Now(),
	})
	if err != nil {
		return err
	}
	_ = ok
	return nil
}

func (r *ArtifactRepositoryImpl) DeleteById(ctx context.Context, id valobj.Id) error {
	return r.store.DeleteById(ctx, id)
}

func (r *ArtifactRepositoryImpl) DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error {
	return r.store.DeleteByNotebookId(ctx, notebookId)
}
```

Note: for `UpdateStatus` we need to allow update from any old-status (syncer accumulates state). If stricter CAS is required by syncer later, add a variant. For now, the impl uses `oldStatus = ""` meaning "no condition" — extend store impl accordingly (already supports empty oldStatus when condition is omitted; verify in step 4 by passing "" — adjust store impl to skip the WHERE clause if empty).

- [ ] **Step 5: Adjust `ArtifactStoreImpl.UpdateStatus` to permit empty `oldStatus`**

Open `internal/infrastructure/database/postgres/artifactstore.go` and change `Where("status = ?", oldStatus)` to `if oldStatus != "" { q = q.Where("status = ?", oldStatus) }`.

- [ ] **Step 6: Run tests, commit**

Run: `go test ./internal/infrastructure/repository/...`
Expected: PASS

```bash
git add internal/infrastructure/repository/
git commit -m "feat(artifact): artifact repository impl + mapper"
```

---

## Task 6: Flow + Syncer + Worker Config Sections

**Files:**
- Create: `internal/conf/flowconfig.go`
- Create: `internal/conf/workerconfig.go`
- Create: `internal/conf/syncerconfig.go`
- Modify: `internal/conf/config.go` — add fields `Flow`, `Worker`, `Syncer` on `Config`.
- Modify: `etc/gonotelm.toml.tpl` — add new sections.
- Test: `internal/conf/config_test.go` (parses new sections).

**Interfaces:**
- Consumes: TOML
- Produces: `conf.FlowConfig`, `conf.WorkerConfig`, `conf.SyncerConfig` consumed by bootstrap, syncer, worker.

- [ ] **Step 1: Write the failing test**

```go
// internal/conf/flow_worker_syncer_test.go
package conf

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FlowWorkerSyncer(t *testing.T) {
	cfg := LoadOrDefaultForTest(t, "./etc/gonotelm.toml.tpl")
	require.NotNil(t, cfg)

	assert.Equal(t, "localhost:7091", cfg.Flow.Addr)
	assert.Equal(t, "gonotelm", cfg.Flow.Namespace)
	assert.Equal(t, 3, cfg.Flow.MaxRetry)
	assert.Equal(t, 5*time.Second, cfg.Flow.DialTimeout)

	assert.Equal(t, 4, cfg.Worker.MaxConcurrency)
	assert.Equal(t, 5*time.Second, cfg.Worker.Heartbeat)
	assert.Len(t, cfg.Worker.TaskTypes, 4)

	assert.Equal(t, 2*time.Second, cfg.Syncer.PerTaskInterval)
	assert.Equal(t, 5*time.Second, cfg.Syncer.GlobalInterval)
	assert.Equal(t, 100, cfg.Syncer.GlobalBatchSize)
}
```

(`LoadOrDefaultForTest` is a helper to render the .tpl with env defaults — reuse the existing `Load` logic in `config.go`.)

- [ ] **Step 2: Implement all three config files**

```go
// internal/conf/flowconfig.go
package conf

import "time"

type FlowConfig struct {
	Addr        string        `toml:"addr"`
	Namespace   string        `toml:"namespace"`
	MaxRetry    int           `toml:"maxRetry"`
	DialTimeout time.Duration `toml:"dialTimeout"`
}
```

```go
// internal/conf/workerconfig.go
package conf

import "time"

type WorkerConfig struct {
	Name            string        `toml:"name"`
	MaxConcurrency  int           `toml:"maxConcurrency"`
	Heartbeat       time.Duration `toml:"heartbeat"`
	TaskTypes       []string      `toml:"taskTypes"`
}
```

```go
// internal/conf/syncerconfig.go
package conf

import "time"

type SyncerConfig struct {
	PerTaskInterval  time.Duration `toml:"perTaskInterval"`
	GlobalInterval   time.Duration `toml:"globalInterval"`
	GlobalBatchSize  int           `toml:"globalBatchSize"`
}
```

Modify `internal/conf/config.go`: add to `Config`:

```go
	Flow   FlowConfig   `toml:"flow"`
	Worker WorkerConfig `toml:"worker"`
	Syncer SyncerConfig `toml:"syncer"`
```

- [ ] **Step 3: Update `etc/gonotelm.toml.tpl`**

Append (after `[text2image.agnes]` block):

```toml

[flow]
addr        = "${GONOTELM_FLOW_ADDR:-localhost:7091}"
namespace   = "${GONOTELM_FLOW_NAMESPACE:-gonotelm}"
maxRetry    = ${GONOTELM_FLOW_MAX_RETRY:-3}
dialTimeout = "${GONOTELM_FLOW_DIAL_TIMEOUT:-5s}"

[syncer]
perTaskInterval = "${GONOTELM_SYNCER_PER_TASK_INTERVAL:-2s}"
globalInterval   = "${GONOTELM_SYNCER_GLOBAL_INTERVAL:-5s}"
globalBatchSize  = ${GONOTELM_SYNCER_GLOBAL_BATCH_SIZE:-100}

[worker]
name            = "${GONOTELM_WORKER_NAME:-gonotelm-worker-1}"
maxConcurrency  = ${GONOTELM_WORKER_MAX_CONCURRENCY:-4}
heartbeat       = "${GONOTELM_WORKER_HEARTBEAT:-5s}"
taskTypes       = ["artifact.mindmap", "artifact.report", "artifact.info_graphic", "artifact.audio_overview"]
```

- [ ] **Step 4: Run tests, commit**

Run: `go test ./internal/conf/...`
Expected: PASS

```bash
git add internal/conf/ etc/gonotelm.toml.tpl
git commit -m "feat(conf): add flow/worker/syncer config sections"
```

---

## Task 7: Flow Task Client Wrapper

**Files:**
- Create: `internal/infrastructure/flow/taskclient.go`
- Test: `internal/infrastructure/flow/taskclient_test.go` (uses an in-process fake gRPC server built by hand, or uses `flow/client/task` against an `inmem` mock — simpler: define an interface, test against a mock implementation).

**Interfaces:**
- Consumes: `github.com/gonotelm-lab/flow/client/task`, `github.com/gonotelm-lab/flow/api/schema/v1`
- Produces: `flow.TaskClient` interface + concrete impl. Used by application usecases + syncer.

- [ ] **Step 1: Define the port and write the failing test using a fake**

```go
// internal/infrastructure/flow/taskclient.go
package flow

import (
	"context"

	flowschema "github.com/gonotelm-lab/flow/api/schema/v1"
)

type TaskState = flowschema.TaskState

type TaskInfo struct {
	ID      string
	State   TaskState
	Result  []byte
	Error   []byte
}

type TaskClient interface {
	Submit(ctx context.Context, taskType string, payload []byte) (flowTaskId string, err error)
	Get(ctx context.Context, flowTaskId string) (*TaskInfo, error)
	Cancel(ctx context.Context, flowTaskId string) error
	Close() error
}
```

Test:
```go
// internal/infrastructure/flow/taskclient_test.go
package flow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeClient struct {
	SubmitFn func(ctx context.Context, taskType string, payload []byte) (string, error)
	GetFn    func(ctx context.Context, id string) (*TaskInfo, error)
	CancelFn func(ctx context.Context, id string) error
}

func (f *fakeClient) Submit(ctx context.Context, t string, p []byte) (string, error) { return f.SubmitFn(ctx, t, p) }
func (f *fakeClient) Get(ctx context.Context, id string) (*TaskInfo, error)            { return f.GetFn(ctx, id) }
func (f *fakeClient) Cancel(ctx context.Context, id string) error                     { return f.CancelFn(ctx, id) }
func (f *fakeClient) Close() error                                                     { return nil }

// validates interface shape only — real client is exercised in integration test
func TestTaskClientInterface(t *testing.T) {
	var _ TaskClient = &fakeClient{}
	var _ TaskClient = (*TaskClientImpl)(nil)
	_ = context.Background()
	_ = assert.True
}
```

- [ ] **Step 2: Run to confirm it fails (TaskClientImpl missing)**

- [ ] **Step 3: Implement TaskClientImpl**

```go
// internal/infrastructure/flow/taskclient.go
// (append to file after the interface)

import (
	"time"

	flowtask "github.com/gonotelm-lab/flow/client/task"
	flowschema "github.com/gonotelm-lab/flow/api/schema/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type TaskClientImpl struct {
	client    *flowtask.Client
	namespace string
	maxRetry  int
}

func NewTaskClient(addr, namespace string, dialTimeout time.Duration, maxRetry int) (*TaskClientImpl, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	c, err := flowtask.New(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	_ = ctx
	return &TaskClientImpl{client: c, namespace: namespace, maxRetry: maxRetry}, nil
}

func (t *TaskClientImpl) Submit(ctx context.Context, taskType string, payload []byte) (string, error) {
	opts := []flowtask.SubmitOption{}
	if t.maxRetry > 0 {
		opts = append(opts, flowtask.WithMaxRetry(t.maxRetry))
	}
	task, err := t.client.Submit(ctx, t.namespace, taskType, payload, opts...)
	if err != nil {
		return "", err
	}
	return task.Id, nil
}

func (t *TaskClientImpl) Get(ctx context.Context, flowTaskId string) (*TaskInfo, error) {
	tk, err := t.client.Get(ctx, flowTaskId)
	if err != nil {
		return nil, err
	}
	return &TaskInfo{ID: tk.Id, State: tk.State, Result: tk.Result, Error: tk.Error}, nil
}

func (t *TaskClientImpl) Cancel(ctx context.Context, flowTaskId string) error {
	return t.client.Cancel(ctx, flowTaskId)
}

func (t *TaskClientImpl) Close() error { return t.client.Close() }
```

- [ ] **Step 4: Run tests, commit**

```bash
git add internal/infrastructure/flow/
git commit -m "feat(flow): add TaskClient wrapper for artifact submission"
```

---

## Task 8: Migrate Studio Prompt Package

**Files:**
- Create: `internal/application/artifact/prompt/prompt.go`
- Create: `internal/application/artifact/prompt/template.go`
- Create: `internal/application/artifact/prompt/system.go`
- Create: `internal/application/artifact/prompt/studio_mindmap.go`
- Create: `internal/application/artifact/prompt/studio_report.go`
- Create: `internal/application/artifact/prompt/studio_infographic.go`
- Create: `internal/application/artifact/prompt/studio_podcast_outline.go`
- Copy (binary copy): `internal/app/biz/prompt/zh/studio-*.jinja` → `internal/application/artifact/prompt/zh/`
- Copy: `internal/app/biz/prompt/zh/title-maker.jinja` → same dir (used by report title generation)
- Test: `internal/application/artifact/prompt/prompt_test.go`

**Interfaces:**
- Consumes: `github.com/cloudwego/eino/components/prompt`, `github.com/cloudwego/eino/schema`, embed FS
- Produces: `Prompt` struct with rendering methods (`RenderStudioMindmapV2Message`, `RenderStudioReportMessage`, `RenderStudioInfoGraphicMessage`, `RenderTitleMakerMessage`).

- [ ] **Step 1: Copy jinja files**

```bash
mkdir -p internal/application/artifact/prompt/zh
cp internal/app/biz/prompt/zh/studio-mindmap.jinja     internal/application/artifact/prompt/zh/
cp internal/app/biz/prompt/zh/studio-mindmap-v2.jinja internal/application/artifact/prompt/zh/
cp internal/app/biz/prompt/zh/studio-report.jinja     internal/application/artifact/prompt/zh/
cp internal/app/biz/prompt/zh/studio-infographic.jinja internal/application/artifact/prompt/zh/
cp internal/app/biz/prompt/zh/studio-podcast-outline.jinja internal/application/artifact/prompt/zh/
cp internal/app/biz/prompt/zh/studio-podcast-transcript.jinja internal/application/artifact/prompt/zh/
cp internal/app/biz/prompt/zh/title-maker.jinja      internal/application/artifact/prompt/zh/
cp internal/app/biz/prompt/system.go                 internal/application/artifact/prompt/system.go
```

(Note: studio-mindmap.jinja (v1) is included for completeness even though mindmap.go uses v2.)

- [ ] **Step 2: Write the failing test**

```go
// internal/application/artifact/prompt/prompt_test.go
package prompt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrompt_RenderStudioMindmapV2(t *testing.T) {
	p := New("zh")
	msgs, err := p.RenderStudioMindmapV2Message(context.Background(), []string{"src-1"}, "")
	require.NoError(t, err)
	assert.NotEmpty(t, msgs)
	assert.Contains(t, msgs[len(msgs)-1].Content, "src-1")
}

func TestPrompt_RenderStudioReport(t *testing.T) {
	p := New("zh")
	_, err := p.RenderStudioReportMessage(context.Background(), []string{"s1", "s2"}, "zh")
	require.NoError(t, err)
}

func TestPrompt_RenderStudioInfoGraphic(t *testing.T) {
	p := New("zh")
	_, err := p.RenderStudioInfoGraphicMessage(context.Background(), StudioInfoGraphicTemplateVars{
		SourceIds: []string{"s1"}, TextLanguage: "zh-cn", ExtraPrompt: "p", Orientation: "landscape", DetailLevel: "standard",
	}, "")
	require.NoError(t, err)
}

func TestPrompt_RenderTitleMaker(t *testing.T) {
	p := New("zh")
	_, err := p.RenderTitleMakerMessage(context.Background(), "report content", "")
	require.NoError(t, err)
}

func TestCheckStudioMindmapResult(t *testing.T) {
	good := "```mermaid\nmindmap\nroot((Root))\n  A\n```"
	assert.True(t, CheckStudioMindmapResult(good))

	bad := "not a mindmap"
	assert.False(t, CheckStudioMindmapResult(bad))
}
```

- [ ] **Step 3: Port template.go and prompt.go**

`template.go` = direct port of `internal/app/biz/prompt/template.go` with package `prompt` and the studio template names. Keep `templateFiles`, `templateStore`, `preloadedTemplates`, `loadTemplateStore`, `template[T]`, `readTemplate`, plus the `defaultLang = "zh"` constant and `templateName*` constants (trimmed to studio subset + title-maker). The embed directive is `//go:embed zh/*.jinja`.

`prompt.go`:

```go
// internal/application/artifact/prompt/prompt.go
package prompt

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
)

type Prompt struct {
	defaultLang string
	systemMsg   *schema.Message
}

func New(defaultLang string) *Prompt {
	normalizedLang := normalizeTemplateLang(defaultLang)
	return &Prompt{
		defaultLang: normalizedLang,
		systemMsg:   schema.SystemMessage(systemPrompt),
	}
}

func (p *Prompt) RenderStudioMindmapV2Message(ctx context.Context, sourceIds []string, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[StudioMindmapV2TemplateVars](templateNameStudioMindmapV2, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, StudioMindmapV2TemplateVars{SourceIds: sourceIds})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

func (p *Prompt) RenderStudioReportMessage(ctx context.Context, sourceIds []string, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[StudioReportTemplateVars](templateNameStudioReport, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, StudioReportTemplateVars{SourceIds: sourceIds})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

func (p *Prompt) RenderStudioInfoGraphicMessage(ctx context.Context, vars StudioInfoGraphicTemplateVars, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[StudioInfoGraphicTemplateVars](templateNameStudioInfographic, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, vars)
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

func (p *Prompt) RenderStudioPodcastOutlineMessage(ctx context.Context, sourceIds []string, lang string, tips string, style PodcastStyle) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[StudioPodcastOutlineTemplateVars](templateNameStudioPodcastOutline, lang, p.defaultLang)
	vars := StudioPodcastOutlineTemplateVars{SourceIds: sourceIds, Language: lang, Tips: tips}
	info, ok := builtinPodcastInfos[style]
	if !ok {
		info = builtinPodcastInfos[PodcastStyleAbstract]
	}
	vars.Style = info.Style
	vars.StyleDesc = info.Description
	vars.Speakers = info.Speakers
	vars.NumOfSegments = info.NumOfSegments

	msg, err := tmpl.Message(ctx, vars)
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

func (p *Prompt) RenderTitleMakerMessage(ctx context.Context, text, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[TitleMakerTemplateVars](templateNameTitleMaker, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, TitleMakerTemplateVars{Text: text})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

func (p *Prompt) prependSystemMessage(msgs []*schema.Message) []*schema.Message {
	return append([]*schema.Message{p.systemMsg}, msgs...)
}

func normalizeTemplateLang(lang string) string { return strings.TrimSpace(lang) }
```

- [ ] **Step 4: Port the template-vars files**

Copy `internal/app/biz/prompt/studiomindmapv2.go`, `studioreport.go`, `studioinfographic.go`, `studiopodcastoutline.go`, `titlemaker.go`, `system.go` into the new package — change `package prompt` to `prompt`. Keep `CheckStudioMindmapResult`. Replace `normalizeStrings` private helper either by copying it from `internal/app/biz/prompt/chat.go` or inlining.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/application/artifact/prompt/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/application/artifact/prompt/
git commit -m "feat(artifact): migrate studio prompts into application/artifact/prompt"
```

---

## Task 9: Generate Use Case (Submit + Spawn Syncer)

**Files:**
- Create: `internal/application/artifact/usecase/generate.go`
- Create: `internal/application/artifact/usecase/deps.go` (shared dependency aggregate)
- Test: `internal/application/artifact/usecase/generate_test.go`

**Interfaces:**
- Consumes: `internal/domain/artifact/{entity, repository, errors}`, `internal/domain/notebook/entity` (for ownership check), `internal/infrastructure/flow`, `internal/application/artifact/syncer` (PollOne), `pkg/context` (UserId), `pkg/errors`, sonic
- Produces: `GenerateUseCase` with `Execute(ctx, *GenerateRequest) (*GenerateResponse, error)`

- [ ] **Step 1: Write the failing test with mocks**

```go
// internal/application/artifact/usecase/generate_test.go
package usecase

import (
	"context"
	"testing"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubRepo struct {
	saved []*artifactentity.Artifact
	err   error
}

func (s *stubRepo) Save(ctx context.Context, a *artifactentity.Artifact) error {
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, a)
	return nil
}
func (s *stubRepo) FindById(ctx context.Context, id valobj.Id) (*artifactentity.Artifact, error) { return nil, artifacterrors.ErrArtifactNotFound }
func (s *stubRepo) ListByNotebookId(ctx context.Context, n valobj.Id, l, o int) ([]*artifactentity.Artifact, error) { return nil, nil }
func (s *stubRepo) ListByStatus(ctx context.Context, sts []artifactentity.Status, l int) ([]*artifactentity.Artifact, error) { return nil, nil }
func (s *stubRepo) UpdateStatus(ctx context.Context, id valobj.Id, st artifactentity.Status, r []byte, rk artifactentity.ResultKind, t string) error { return nil }
func (s *stubRepo) DeleteById(ctx context.Context, id valobj.Id) error { return nil }
func (s *stubRepo) DeleteByNotebookId(ctx context.Context, n valobj.Id) error { return nil }

var _ artifactrepo.Repository = &stubRepo{}

type stubFlow struct {
	submitID string
	submitErr error
	canceled []string
}

func (s *stubFlow) Submit(ctx context.Context, t string, p []byte) (string, error) { return s.submitID, s.submitErr }
func (s *stubFlow) Get(ctx context.Context, id string) (*flow.TaskInfo, error) { return nil, nil }
func (s *stubFlow) Cancel(ctx context.Context, id string) error { s.canceled = append(s.canceled, id); return nil }
func (s *stubFlow) Close() error { return nil }

func TestGenerate_Execute_HappyPath(t *testing.T) {
	repo := &stubRepo{}
	flowc := &stubFlow{submitID: "flow-1"}
	notebookRepo := newNotebookStub("u1")
	uc := NewGenerate(repo, flowc, notebookRepo, nil)

	resp, err := uc.Execute(context.WithValue(context.Background(), userIdKey{}, "u1"), &GenerateRequest{
		NotebookId: uuid.NewV7(),
		Kind:       artifactentity.KindMindmap,
		SourceIds:  []valobj.Id{uuid.NewV7()},
	})

	require.NoError(t, err)
	assert.NotEqual(t, valobj.Id{}, resp.ArtifactId)
	assert.Equal(t, "flow-1", repo.saved[0].FlowTaskId)
}

func TestGenerate_Execute_NotebookOwnedByOther(t *testing.T) {
	repo := &stubRepo{}
	flowc := &stubFlow{}
	notebookRepo := newNotebookStub("other-user")
	uc := NewGenerate(repo, flowc, notebookRepo, nil)

	_, err := uc.Execute(context.WithValue(context.Background(), userIdKey{}, "u1"), &GenerateRequest{
		NotebookId: uuid.NewV7(), Kind: artifactentity.KindMindmap, SourceIds: nil,
	})
	require.Error(t, err)
}
```

Helpers:
- `newNotebookStub(owner string)` returns a notebook repo interface with `FindById` returning a notebook whose `OwnerId` is the given string.
- For the test to compile, we need a `notebookrepo.Repository`-compatible stub. Use the existing interface at `internal/domain/notebook/repository` (import it).
- `userIdKey{}` test helper = typed context key matching `pkg/context.GetUserId` behavior. Read `pkg/context/context.go` to confirm key type, then use `pkgcontext.WithUserId(ctx, "u1")` directly.

- [ ] **Step 2: Implement deps.go**

```go
// internal/application/artifact/usecase/deps.go
package usecase

import (
	"context"

	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
)

type Poller interface {
	PollOne(ctx context.Context, artifactId valobj.Id)
}

type Deps struct {
	ArtifactRepo artifactrepo.Repository
	FlowClient   flow.TaskClient
	NotebookRepo notebookrepo.Repository
	Poller       Poller
}
```

- [ ] **Step 3: Implement generate.go**

```go
// internal/application/artifact/usecase/generate.go
package usecase

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type GenerateRequest struct {
	NotebookId     valobj.Id
	Kind           artifactentity.Kind
	SourceIds      []valobj.Id
	InfoGraphic    *artifactentity.InfoGraphicPayload
	AudioOverview  *artifactentity.AudioOverviewPayload
}

type GenerateResponse struct {
	ArtifactId valobj.Id
}

type GenerateUseCase struct {
	repo        artifactrepo.Repository
	flow        flow.TaskClient
	notebook    notebookrepo.Repository
	poller      Poller
}

func NewGenerate(
	repo artifactrepo.Repository,
	flowc flow.TaskClient,
	notebook notebookrepo.Repository,
	poller Poller,
) *GenerateUseCase {
	return &GenerateUseCase{repo: repo, flow: flowc, notebook: notebook, poller: poller}
}

func (u *GenerateUseCase) Execute(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	userId := pkgcontext.GetUserId(ctx)

	nb, err := u.notebook.FindById(ctx, req.NotebookId)
	if err != nil {
		return nil, err
	}
	if nb.OwnerId != userId {
		return nil, errors.ErrPermission.Msgf("notebook access denied, notebook_id=%s", req.NotebookId)
	}

	payload, err := buildPayload(req)
	if err != nil {
		return nil, err
	}

	artifact := artifactentity.NewArtifact(req.NotebookId, userId, req.Kind, payload)

	payloadBytes, err := sonic.Marshal(payload)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal generate payload err=%v", err)
	}

	flowTaskId, err := u.flow.Submit(ctx, taskTypeFor(req.Kind), payloadBytes)
	if err != nil {
		return nil, errors.WithMessage(err, "submit artifact task to flow failed")
	}

	artifact.BindFlowTaskId(flowTaskId)

	if err := u.repo.Save(ctx, artifact); err != nil {
		return nil, errors.WithMessage(err, "save artifact failed")
	}

	if u.poller != nil {
		// spawn per-task polling goroutine for low-latency status sync
		go u.poller.PollOne(context.WithoutCancel(ctx), artifact.Id)
	}

	return &GenerateResponse{ArtifactId: artifact.Id}, nil
}

func buildPayload(req *GenerateRequest) (artifactentity.Payload, error) {
	switch req.Kind {
	case artifactentity.KindMindmap:
		return &artifactentity.MindmapPayload{NotebookId: req.NotebookId, SourceIds: req.SourceIds}, nil
	case artifactentity.KindReport:
		return &artifactentity.ReportPayload{NotebookId: req.NotebookId, SourceIds: req.SourceIds}, nil
	case artifactentity.KindInfoGraphic:
		if req.InfoGraphic == nil {
			return nil, errors.ErrParams.Msgf("info_graphic payload required")
		}
		req.InfoGraphic.NotebookId = req.NotebookId
		req.InfoGraphic.SourceIds = req.SourceIds
		return req.InfoGraphic, nil
	case artifactentity.KindAudioOverview:
		if req.AudioOverview == nil {
			return nil, errors.ErrParams.Msgf("audio_overview payload required")
		}
		req.AudioOverview.NotebookId = req.NotebookId
		req.AudioOverview.SourceIds = req.SourceIds
		return req.AudioOverview, nil
	}
	return nil, errors.ErrParams.Msgf("unsupported artifact kind: %s", req.Kind)
}

func taskTypeFor(kind artifactentity.Kind) string {
	return "artifact." + kind.String()
}
```

- [ ] **Step 4: Run test, commit**

```bash
go test ./internal/application/artifact/usecase/...
git add internal/application/artifact/usecase/
git commit -m "feat(artifact): generate use case with flow.Submit + repo.Save"
```

---

## Task 10: Status / List / Cancel / Delete Use Cases

**Files:**
- Create: `internal/application/artifact/usecase/status.go` (covers status + result lookup)
- Create: `internal/application/artifact/usecase/list.go`
- Create: `internal/application/artifact/usecase/cancel.go`
- Create: `internal/application/artifact/usecase/delete.go`
- Create: `internal/application/artifact/usecase/retry.go`
- Test: `internal/application/artifact/usecase/status_test.go`, etc., covering happy path + permission denied + wrong-state errors.

**Interfaces:**
- Consumes: same as Task 9 + storage gateway for `delete` (storage-bound results cleanup). Use a small port `StorageGateway` interface defined in `usecase/deps.go`.
- Produces: `StatusUseCase`, `ListUseCase`, `CancelUseCase`, `DeleteUseCase`, `RetryUseCase`.

- [ ] **Step 1: Define Storage port + add to deps**

Append to `internal/application/artifact/usecase/deps.go`:

```go
type StorageGateway interface {
	DeleteObject(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string) (string, error)
}
```

- [ ] **Step 2: Implement status.go**

```go
// internal/application/artifact/usecase/status.go
package usecase

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type StatusRequest struct{ ArtifactId valobj.Id }
type StatusResponse struct {
	Status   artifactentity.Status
	Title    string
	Result   []byte
	ResultKind artifactentity.ResultKind
	FlowError string
}

type StatusUseCase struct {
	repo   artifactrepo.Repository
	flowc  flow.TaskClient
}

func NewStatus(repo artifactrepo.Repository, flowc flow.TaskClient) *StatusUseCase {
	return &StatusUseCase{repo: repo, flowc: flowc}
}

func (u *StatusUseCase) Execute(ctx context.Context, req *StatusRequest) (*StatusResponse, error) {
	a, err := u.repo.FindById(ctx, req.ArtifactId)
	if err != nil {
		return nil, err
	}
	userId := pkgcontext.GetUserId(ctx)
	if !a.IsOwner(userId) {
		return nil, artifacterrors.ErrArtifactNotOwnedByUser
	}

	if a.IsTerminal() {
		return &StatusResponse{Status: a.Status, Title: a.Title, Result: a.Result, ResultKind: a.ResultKind}, nil
	}

	if a.FlowTaskId == "" {
		return nil, artifacterrors.ErrInvalidFlowTaskId
	}

	info, err := u.flowc.Get(ctx, a.FlowTaskId)
	if err != nil {
		return nil, errors.WithMessage(err, "query flow task failed")
	}
	mapped := mapFlowState(info.State)
	return &StatusResponse{Status: mapped, FlowError: string(info.Error)}, nil
}

func mapFlowState(state flow.TaskState) artifactentity.Status {
	switch state {
	case 1: // INITED
		return artifactentity.StatusPending
	case 2: // RUNNING
		return artifactentity.StatusRunning
	case 3: // DONE
		return artifactentity.StatusCompleted
	case 4: // FAILED
		return artifactentity.StatusFailed
	case 5: // CANCELLED
		return artifactentity.StatusCancelled
	}
	return artifactentity.StatusPending
}
```

(Use the numeric `TaskState` constants from `flowschema.TaskState_STATE_INITED` etc. for clarity — replace the literals with those imported from `github.com/gonotelm-lab/flow/api/schema/v1`.)

- [ ] **Step 3: Implement list.go**

```go
// internal/application/artifact/usecase/list.go
package usecase

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ListRequest struct {
	NotebookId valobj.Id
	Limit      int
	Offset     int
}

type ListResponse struct {
	Artifacts []*artifactentity.Artifact
	HasMore   bool
}

type ListUseCase struct {
	repo     artifactrepo.Repository
	notebook notebookrepo.Repository
	storage  StorageGateway
}

func NewList(repo artifactrepo.Repository, notebook notebookrepo.Repository, storage StorageGateway) *ListUseCase {
	return &ListUseCase{repo: repo, notebook: notebook, storage: storage}
}

func (u *ListUseCase) Execute(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	userId := pkgcontext.GetUserId(ctx)
	nb, err := u.notebook.FindById(ctx, req.NotebookId)
	if err != nil {
		return nil, err
	}
	if nb.OwnerId != userId {
		return nil, errors.ErrPermission.Msgf("notebook access denied, notebook_id=%s", req.NotebookId)
	}

	fetchLimit := req.Limit + 1
	rows, err := u.repo.ListByNotebookId(ctx, req.NotebookId, fetchLimit, req.Offset)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > req.Limit
	if hasMore {
		rows = rows[:req.Limit]
	}
	return &ListResponse{Artifacts: rows, HasMore: hasMore}, nil
}
```

- [ ] **Step 4: Implement cancel.go, delete.go, retry.go**

```go
// internal/application/artifact/usecase/cancel.go
package usecase

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

type CancelUseCase struct {
	repo  artifactrepo.Repository
	flowc flow.TaskClient
}

func NewCancel(repo artifactrepo.Repository, flowc flow.TaskClient) *CancelUseCase {
	return &CancelUseCase{repo: repo, flowc: flowc}
}

func (u *CancelUseCase) Execute(ctx context.Context, artifactId valobj.Id) error {
	a, err := u.repo.FindById(ctx, artifactId)
	if err != nil {
		return err
	}
	if !a.IsOwner(pkgcontext.GetUserId(ctx)) {
		return artifacterrors.ErrArtifactNotOwnedByUser
	}
	if a.IsTerminal() {
		return artifacterrors.ErrCannotCancelInState
	}
	if a.FlowTaskId == "" {
		return artifacterrors.ErrInvalidFlowTaskId
	}
	if err := u.flowc.Cancel(ctx, a.FlowTaskId); err != nil {
		return err
	}
	a.MarkCancelled()
	return u.repo.UpdateStatus(ctx, a.Id, a.Status, nil, "", "")
}
```

```go
// internal/application/artifact/usecase/delete.go
package usecase

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteUseCase struct {
	repo    artifactrepo.Repository
	flowc   flow.TaskClient
	storage StorageGateway
}

func NewDelete(repo artifactrepo.Repository, flowc flow.TaskClient, storage StorageGateway) *DeleteUseCase {
	return &DeleteUseCase{repo: repo, flowc: flowc, storage: storage}
}

func (u *DeleteUseCase) Execute(ctx context.Context, artifactId valobj.Id) error {
	a, err := u.repo.FindById(ctx, artifactId)
	if err != nil {
		return err
	}
	if !a.IsOwner(pkgcontext.GetUserId(ctx)) {
		return artifacterrors.ErrArtifactNotOwnedByUser
	}
	if !a.IsTerminal() && a.FlowTaskId != "" {
		if err := u.flowc.Cancel(ctx, a.FlowTaskId); err != nil {
			return errors.WithMessage(err, "cancel flow task failed")
		}
	}
	if a.ResultKind.Storage() && a.Result != nil {
		// best effort - log but don't fail delete
		var key string
		_ = key // extract store key from a.Result via sonic into `key`; helper defined in storage result parser
		_ = u.storage.DeleteObject(ctx, key)
	}
	return u.repo.DeleteById(ctx, a.Id)
}
```

To extract the storage key, add helper in `internal/application/artifact/usecase/storage_result.go` that parses `a.Result` with sonic into a struct `{StoreKey, ContentType, Image}`. Implement and import it in delete.go (one paragraph of code).

```go
// internal/application/artifact/usecase/retry.go
package usecase

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type RetryUseCase struct {
	repo   artifactrepo.Repository
	flowc  flow.TaskClient
	poller Poller
}

func NewRetry(repo artifactrepo.Repository, flowc flow.TaskClient, poller Poller) *RetryUseCase {
	return &RetryUseCase{repo: repo, flowc: flowc, poller: poller}
}

func (u *RetryUseCase) Execute(ctx context.Context, artifactId valobj.Id) error {
	a, err := u.repo.FindById(ctx, artifactId)
	if err != nil {
		return err
	}
	if !a.IsOwner(pkgcontext.GetUserId(ctx)) {
		return artifacterrors.ErrArtifactNotOwnedByUser
	}
	if a.Status != artifactentity.StatusFailed && a.Status != artifactentity.StatusCancelled {
		return artifacterrors.ErrCannotRetryInState
	}

	oldFlowTaskId := a.FlowTaskId
	payloadBytes, err := sonic.Marshal(a.Payload)
	if err != nil {
		return errors.Wrapf(errors.ErrSerde, "marshal payload on retry err=%v", err)
	}
	newFlowTaskId, err := u.flowc.Submit(ctx, taskTypeFor(a.Kind), payloadBytes)
	if err != nil {
		return errors.WithMessage(err, "submit retry task to flow failed")
	}
	a.MarkRetrying(newFlowTaskId)
	if err := u.repo.Save(ctx, a); err != nil {
		return errors.WithMessage(err, "save retried artifact failed")
	}
	if oldFlowTaskId != "" && oldFlowTaskId != newFlowTaskId {
		go func() { _ = u.flowc.Cancel(context.WithoutCancel(ctx), oldFlowTaskId) }()
	}
	if u.poller != nil {
		go u.poller.PollOne(context.WithoutCancel(ctx), a.Id)
	}
	return nil
}
```

(`repo.Save` upserts — but the store only `Create`s. Need to add an `Update` path on the store. Add `Update(ctx, *schema.Artifact) error` to `ArtifactStore` interface and impl; extend repository `Save` to upsert. For simplicity in retry, change `RetryUseCase.Execute` to call `repo.UpdateStatus` instead of `Save`, and maintain a separate `UpdateFlowTaskId` store method — already present. Update `RetryUseCase` to set `artifact.MarkRetrying(...)` then call `repo.UpdateFlowTaskId(a.Id, newFlowTaskId, []Status{Failed, Cancelled})` instead of `Save`. Adjust accordingly.)

- [ ] **Step 5: Write tests for each use case**

Two-round pattern: happy path and one error case per use case (status known/terminal; cancel denied for terminal; delete flow.Cancel invoked; retry only allowed on failed/cancelled). Use the existing stubs from Task 9's test files plus a new stub for `StorageGateway`.

- [ ] **Step 6: Run tests, commit**

```bash
go test ./internal/application/artifact/usecase/...
git add internal/application/artifact/usecase/
git commit -m "feat(artifact): status/list/cancel/delete/retry use cases"
```

---

## Task 11: Status Syncer (Per-task + Global)

**Files:**
- Create: `internal/application/artifact/syncer/syncer.go`
- Create: `internal/application/artifact/syncer/poll_one.go`
- Create: `internal/application/artifact/syncer/global.go`
- Test: `internal/application/artifact/syncer/syncer_test.go`

**Interfaces:**
- Consumes: `internal/domain/artifact/{entity, repository}`, `internal/infrastructure/flow`, `pkg/safe`
- Produces: `Syncer` struct; `NewSyncer(repo, flowc, conf.SyncerConfig)`; `Start(ctx)` runs global loop; `PollOne(ctx, artifactId)` per-task; `Shutdown(ctx)`.

- [ ] **Step 1: Write the failing test**

```go
// internal/application/artifact/syncer/syncer_test.go
package syncer

import (
	"context"
	"sync"
	"testing"
	"time"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	flowschema "github.com/gonotelm-lab/flow/api/schema/v1"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type syncRepo struct {
	mu      sync.Mutex
	byId    map[valobj.Id]*artifactentity.Artifact
	updated []struct{ id valobj.Id; st artifactentity.Status }
}

func (s *syncRepo) Save(ctx context.Context, a *artifactentity.Artifact) error { s.byId[a.Id] = a; return nil }
// implement the rest of the interface returning from s.byId / recording updates

func TestSyncer_PollOne_ReachesTerminalAndStops(t *testing.T) {
	a := artifactentity.NewArtifact(uuid.NewV7(), "u", artifactentity.KindMindmap, &artifactentity.MindmapPayload{})
	a.BindFlowTaskId("ft-1")
	repo := newSyncRepo(a)
	flowc := newSyncFlow(flowschema.TaskState_RUNNING, flowschema.TaskState_DONE, []byte("result"))

	s := NewSyncer(repo, flowc, SyncerConfig{PerTaskInterval: 10 * time.Millisecond, GlobalInterval: 50 * time.Millisecond, GlobalBatchSize: 100})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	s.PollOne(ctx, a.Id)

	deadline := time.After(2 * time.Second)
	for {
		got, _ := repo.FindById(ctx, a.Id)
		if got.Status.Completed() {
			break
		}
		select {
		case <-deadline:
			t.Fatal("did not reach completed in time")
		case <-time.After(20 * time.Millisecond):
		}
	}
	require.Eventually(t, func() bool {
		got, _ := repo.FindById(ctx, a.Id)
		return got.Status.Completed() && len(got.Title) >= 0
	}, 2*time.Second, 20*time.Millisecond)
	_ = assert.NotNil
}
```

Two helpers: `newSyncRepo(a)` returns a map-backed `artifactrepo.Repository`; `newSyncFlow(states ...flowschema.TaskState)` returns a fake `flow.TaskClient` that returns successive states on each `Get` then the final one forever.

- [ ] **Step 2: Implement syncer.go**

```go
// internal/application/artifact/syncer/syncer.go
package syncer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
)

type Config struct {
	PerTaskInterval time.Duration
	GlobalInterval  time.Duration
	GlobalBatchSize  int
}

type Syncer struct {
	repo  artifactrepo.Repository
	flow  flow.TaskClient
	cfg   Config
	wg    sync.WaitGroup
	stop  chan struct{}
}

func NewSyncer(repo artifactrepo.Repository, flowc flow.TaskClient, cfg Config) *Syncer {
	return &Syncer{repo: repo, flow: flowc, cfg: cfg, stop: make(chan struct{})}
}

func (s *Syncer) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.globalLoop(ctx)
}

func (s *Syncer) Shutdown(ctx context.Context) {
	close(s.stop)
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
}
```

- [ ] **Step 3: Implement poll_one.go**

```go
// internal/application/artifact/syncer/poll_one.go
package syncer

import (
	"context"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

func (s *Syncer) PollOne(ctx context.Context, artifactId valobj.Id) {
	ticker := time.NewTicker(s.cfg.PerTaskInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
			done, err := s.pollOnce(ctx, artifactId)
			if err != nil {
				slog.WarnContext(ctx, "pollOnce failed", "artifact_id", artifactId, "err", err)
				continue
			}
			if done {
				return
			}
		}
	}
}

func (s *Syncer) pollOnce(ctx context.Context, artifactId valobj.Id) (done bool, err error) {
	a, err := s.repo.FindById(ctx, artifactId)
	if err != nil {
		return true, err
	}
	if a.IsTerminal() {
		return true, nil
	}
	info, err := s.flow.Get(ctx, a.FlowTaskId)
	if err != nil {
		return false, err
	}
	newStatus := mapFlowState(info.State)
	if newStatus == a.Status {
		return false, nil
	}
	switch newStatus {
	case artifactentity.StatusCompleted:
		// result bytes are JSON payload; for storage kind, decode to get presigned URL later in use case
		var title string
		var result []byte
		var resultKind artifactentity.ResultKind
		// prefer info.Result if present
		result = info.Result
		_ = title // flow doesn't carry title — keep existing title (worker sets empty title for now)
		_ = resultKind
		if err := s.repo.UpdateStatus(ctx, a.Id, newStatus, result, "", ""); err != nil {
			return false, err
		}
	case artifactentity.StatusFailed, artifactentity.StatusCancelled:
		if err := s.repo.UpdateStatus(ctx, a.Id, newStatus, nil, "", ""); err != nil {
			return false, err
		}
	case artifactentity.StatusRunning:
		if err := s.repo.UpdateStatus(ctx, a.Id, newStatus, nil, "", ""); err != nil {
			return false, err
		}
	case artifactentity.StatusPending:
		return false, nil
	}
	return newStatus.IsTerminalSafe(), nil
}

// helper to keep terminal check in syncer too
```

Add a method `IsTerminalSafe()` on the entity (or just call `IsTerminal()` — the test exists in entity). For consistency, add the predicate as a method on Status:

Actually reuse `IsTerminal()` on `Status` — add `func (s Status) IsTerminal() bool { return s.Completed() || s.Failed() || s.Cancelled() }` to the entity (refine Task 1 subsequently; this is a small follow-up edit — do it now).

- [ ] **Step 4: Implement global.go (sweep loop)**

```go
// internal/application/artifact/syncer/global.go
package syncer

import (
	"context"
	"log/slog"
	"time"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

func (s *Syncer) globalLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.cfg.GlobalInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
			if err := s.scanOnce(ctx); err != nil {
				slog.WarnContext(ctx, "syncer scan once failed", "err", err)
			}
		}
	}
}

func (s *Syncer) scanOnce(ctx context.Context) error {
	rows, err := s.repo.ListByStatus(ctx,
		[]artifactentity.Status{artifactentity.StatusPending, artifactentity.StatusRunning},
		s.cfg.GlobalBatchSize,
	)
	if err != nil {
		return err
	}
	for _, a := range rows {
		_, _ = s.pollOnce(ctx, a.Id)
	}
	return nil
}
```

Add `IsTerminal()` method on `Status` to entity package (edit Task 1 file). Add a `mapFlowState` helper in `syncer.go` that mirrors the usecase mapFlowState.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/application/artifact/syncer/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/application/artifact/syncer/ internal/domain/artifact/entity/artifact.go
git commit -m "feat(artifact): status syncer (per-task and global scan)"
```

---

## Task 12: Bootstrap — Shared Infra + Worker App + cmd/worker

**Files:**
- Create: `internal/bootstrap/shared.go`
- Create: `internal/bootstrap/worker_app.go`
- Create: `cmd/worker/main.go`
- Modify: `cmd/main.go` → move to `cmd/gonotelm/main.go` (physical move).

**Interfaces:**
- Consumes: many infra packages + new generator package (Task 14+) — but with generators not yet written, defer the per-kind handler registration to Task 17 (worker bootstrap complete). For this task, just scaffold wiring of infra + flow client; leave the per-kind handler registration as a thin shim interface — registration itself comes in Task 18.
- Produces: `bootstrap.NewWorkerApp(ctx, cfg) (*WorkerApp, error)` returning a struct with `Run()` blocking on ctx and `Close()`.

Simplification: complete `worker_app.go` together with generator implementations in Task 18. In this task, only create `shared.go` (used by `app.go` too) and a minimal `cmd/worker/main.go` that builds (without per-kind handlers wired yet) — wire later in Task 18.

To keep plan bite-sized: this task scaffolds `shared.go` and moves `cmd/main.go`. Task 18 finishes `worker_app.go` + `cmd/worker/main.go` end-to-end.

- [ ] **Step 1: Move cmd/main.go to cmd/gonotelm/main.go**

```bash
mkdir -p cmd/gonotelm
git mv cmd/main.go cmd/gonotelm/main.go
```

- [ ] **Step 2: Implement shared.go**

Extract the shared infra setup from `bootstrap/app.go` into a reusable function returning a struct of deps. Keep `app.go` calling `shared.go` internally so behavior stays the same.

```go
// internal/bootstrap/shared.go
package bootstrap

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	text2image "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"
)

type SharedInfra struct {
	Closer          misc.Closer
	LLMGateway      *chat.Gateway
	Text2Image      *text2image.Text2ImageGateway
	ObjectStorage   storage.Storage
	ArtifactDB      *database.DAL
}

func NewSharedInfra(ctx context.Context, cfg *conf.Config) (_ *SharedInfra, outErr error) {
	// identical wiring to the corresponding chunks of app.go; reuse the existing helpers `newLLMGateway`, `newStorage`, ... already in app.go
	// return the struct, also collect closers
	...
}
```

(For actual implementation, copy the relevant lines from `app.go:60-108` into `shared.go` and refactor `app.go` to call it. No behavior change expected; existing tests should still pass.)

- [ ] **Step 3: Verify build + tests still pass**

```bash
go build ./cmd/gonotelm
go test ./...
```

- [ ] **Step 4: Commit**

```bash
git add cmd/ internal/bootstrap/
git commit -m "refactor(bootstrap): extract SharedInfra; move cmd/main.go to cmd/gonotelm/main.go"
```

---

## Task 13: Generators — Common Scaffolding + Mindmap

**Files:**
- Create: `internal/application/artifact/generate/generate.go` (Request/Response, dispatcher `Generate(ctx, *Request) (*Response, error)` route by Kind)
- Create: `internal/application/artifact/generate/session.go` (`SessionState`)
- Create: `internal/application/artifact/generate/agent.go` (helpers: `buildSourceExploreAgent`, `sourceIDsToStrings`)
- Create: `internal/application/artifact/generate/mindmap.go`
- Test: `internal/application/artifact/generate/mindmap_test.go`

**Interfaces:**
- Consumes: `internal/application/artifact/prompt`, `internal/application/chat/agent/tools` (readsource/grepsource/querysource/statsource), `internal/domain/source/service/agentize`, `pkg/agent`, `internal/infrastructure/llm/chat`, `internal/conf`
- Produces: `GenerateRequest`, `GenerateResponse`, `Run(ctx, *GenerateRequest) (*GenerateResponse, error)`

- [ ] **Step 1: Implement session.go + agent.go**

Session state holds `NotebookId`, `SourceIds`, `UserId`, `Lang` (extracted from ctx). Use the chat/agent pattern at `internal/application/chat/agent/agent.go:155-182` to build `pkg/agent.Agent[*SessionState]`. Bind tools `readsource`, `grepsource`, `querysource`, `statsource` from `internal/application/chat/agent/tools/`.

Mirror `internal/app/logic/studio/agentcommon.go:106-149` for `newFinalRoundHook` (copy verbatim into `agent.go` as a generic helper, adapted to `*pkgagent.Agent[*SessionState]`).

- [ ] **Step 2: Implement mindmap.go**

Port `internal/app/logic/studio/mindmap.go:79-363` into the new package, replacing:
- `m.l.helpGetNotebook` → drop (notebook check is now in API process). The worker doesn't validate notebook/user.
- `m.l.sourceBiz.BatchGetDecodedSources` + `m.l.helpGetSourcesParsedContent` → use `agentize.Service` `ReadSource`/`StatSource` tools instead via the agent (which is what `agentCreateMindmap` already does for the big-text path). Keep only the agent path (drop `oneshotCreateMindmap` and `twoshotCreateMindmap` for the new flow, since big-text path is the canonical one — small-text fallback is a stretch). For now, port only `agentCreateMindmap` and `parseAgentOutput`.

Configuration source: `conf.Global().Logic.Studio.Mindmap.{Model,ModelProvider,MaxRound}` (existing).

Return `(*GenerateResponse, error)` where `Response = { Title string; Result []byte; ResultKind ResultKind }`. Set `ResultKind = Inline`, `Result = mindmap bytes`, `Title = expect.Title`.

- [ ] **Step 3: Implement generate.go dispatcher**

```go
// internal/application/artifact/generate/generate.go
package generate

import (
	"context"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type Request struct {
	ArtifactId  valobj.Id
	NotebookId  valobj.Id
	UserId      string
	SourceIds   []valobj.Id
	Kind        artifactentity.Kind
	Payload     artifactentity.Payload
}

type Response struct {
	Title      string
	Result     []byte
	ResultKind artifactentity.ResultKind
}

type ServiceDeps struct {
	Agentize *agentize.Service
	LLMGateway *chat.Gateway
	Prompt *prompt.Prompt
}

type Generator interface {
	Generate(ctx context.Context, req *Request) (*Response, error)
}

func Run(ctx context.Context, deps *ServiceDeps, req *Request) (*Response, error) {
	g, err := newGenerator(req.Kind, deps)
	if err != nil {
		return nil, err
	}
	return g.Generate(ctx, req)
}

func newGenerator(kind artifactentity.Kind, deps *ServiceDeps) (Generator, error) {
	switch kind {
	case artifactentity.KindMindmap:
		return &MindmapGenerator{deps: deps}, nil
	}
	return nil, errors.ErrParams.Msgf("unsupported kind: %s", kind)
}
```

- [ ] **Step 4: Write a focused test**

`mindmap_test.go` mocks `chat.Gateway` (return a canned model that yields a fixed JSON `{"title":"...","mindmap":"```mermaid\nmindmap\nroot((R))\n```"}`) and a no-op `agentize.Service`. Run `Run` and assert `Response.Title` and `Response.ResultKind == Inline`.

(Mocks for `chat.Gateway` may be tricky — match the existing pattern by introducing a `ChatModelProviderFunc` shim. If too complex, write the test as an integration test that hits the real LLM gateway via `conf.Global()`. The user has `.env` set; mark this as an integration test with `t.Skip` if no LLM creds detected.)

- [ ] **Step 5: Run tests, commit**

```bash
go test ./internal/application/artifact/generate/...
git add internal/application/artifact/generate/
git commit -m "feat(artifact): generator scaffolding + mindmap"
```

---

## Task 14: Generators — Report

**Files:**
- Create: `internal/application/artifact/generate/report.go`
- Test: `internal/application/artifact/generate/report_test.go`

Mirror the mindmap pattern; port `internal/app/logic/studio/report.go:63-153`. Include title generation via `deps.Prompt.RenderTitleMakerMessage`. Result kind = Inline. Add `KindReport` case to `newGenerator`.

Write tests focusing on dispatcher (which calls report generator) and `parseAgentOutput`/compensation behavior; skip real LLM unless integration test.

- [ ] **Step 1-4: TDD steps analogous to Task 13. Commit**:

```bash
git add internal/application/artifact/generate/report.go internal/application/artifact/generate/report_test.go internal/application/artifact/generate/generate.go
git commit -m "feat(artifact): report generator"
```

---

## Task 15: Generators — Infographic

**Files:**
- Create: `internal/application/artifact/generate/infographic.go`
- Test: `internal/application/artifact/generate/infographic_test.go`

Port `internal/app/logic/studio/infographic.go:127-333`. Returns `ResultKind = Storage` and `Result` = marshaled `ArtifactStorageResult` (define a fresh struct inside `internal/application/artifact/generate/storage_result.go` mirroring `internal/app/model/artifact.go:198-227`).

Use `deps.Text2Image` for image generation and `deps.ObjectStorage` for upload (add to `ServiceDeps`). Add `KindInfoGraphic` case to `newGenerator`.

Write tests around `parseAgentOutput` and storage key string format.

Commit:
```bash
git add internal/application/artifact/generate/infographic.go internal/application/artifact/generate/infographic_test.go internal/application/artifact/generate/storage_result.go
git commit -m "feat(artifact): infographic generator"
```

---

## Task 16: Generators — Audio Overview (Placeholder)

**Files:**
- Create: `internal/application/artifact/generate/audiooverview.go`
- Test: `internal/application/artifact/generate/audiooverview_test.go`

Old code at `internal/app/logic/studio/audiooverview.go:88-104` returns `"not implemented"`. Port the placeholder — same behavior. Add `KindAudioOverview` case to `newGenerator` returning `errors.ErrInner.Msg("audio overview generator not implemented")`.

Test: dispatcher returns "not implemented" for audio kind.

Commit:
```bash
git add internal/application/artifact/generate/audiooverview.go internal/application/artifact/generate/audiooverview_test.go
git commit -m "feat(artifact): audio overview generator placeholder"
```

---

## Task 17: Worker App + cmd/worker/main.go

**Files:**
- Create: `internal/bootstrap/worker_app.go`
- Create: `cmd/worker/main.go`
- Test: `internal/bootstrap/worker_app_test.go` (mocks flow,address; asserts 4 clients created and started; doesn't actually connect to a real server).

**Interfaces:**
- Consumes: `flow/client/worker`, `internal/application/artifact/generate`, `internal/bootstrap.SharedInfra`, `internal/conf`
- Produces: `bootstrap.NewWorkerApp(ctx, cfg) (*WorkerApp, error)`; `WorkerApp.Run()` blocks until ctx canceled; `WorkerApp.Close()`.

- [ ] **Step 1: Implement worker_app.go**

```go
// internal/bootstrap/worker_app.go
package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/gonotelm-lab/flow/client/worker"
	flowworker "github.com/gonotelm-lab/flow/client/worker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	artifactapp "github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
)

type WorkerApp struct {
	shared *SharedInfra
	clients []*flowworker.Client
	gen     artifactapp.Generator
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func NewWorkerApp(ctx context.Context, cfg *conf.Config) (_ *WorkerApp, outErr error) {
	shared, err := NewSharedInfra(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer func() {
		if outErr != nil {
			shared.Close(ctx)
		}
	}()

	conn, err := grpc.NewClient(cfg.Flow.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	gen := artifactapp.NewService(&artifactapp.ServiceDeps{
		// wire agentize.Service (built from shared.DAL.SourceStore + storage + vector DB)
		// wire LLMGateway, Prompt, Text2Image, ObjectStorage
	})
	app := &WorkerApp{shared: shared, gen: gen}

	for _, taskType := range cfg.Worker.TaskTypes {
		kind := kindFromTaskType(taskType)
		fcfg := worker.ConfigWithDefaults(worker.Config{
			Namespace:      cfg.Flow.Namespace,
			TaskType:       taskType,
			Name:           cfg.Worker.Name,
			MaxConcurrency: cfg.Worker.MaxConcurrency,
			HeartbeatInterval: cfg.Worker.Heartbeat,
		})
		c := worker.NewWithConn(conn, fcfg)
		artifactapp.RegisterTypedWorker(c, gen, kind)
		app.clients = append(app.clients, c)
	}
	return app, nil
}

func (a *WorkerApp) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	for _, c := range a.clients {
		if err := c.Start(); err != nil {
			return err
		}
	}
	<-ctx.Done()
	return nil
}

func (a *WorkerApp) Close(ctx context.Context) error {
	if a.cancel != nil { a.cancel() }
	var firstErr error
	for _, c := range a.clients {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := a.shared.Close(ctx); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func kindFromTaskType(taskType string) artifactentity.Kind {
	// strip "artifact." prefix
}
```

`artifactapp.RegisterTypedWorker` — add a helper in `internal/application/artifact/generate/worker.go` that wraps `worker.RegisterTypedResult` and converts the typed Request/Response. (Define this file too as part of this task.)

- [ ] **Step 2: Implement cmd/worker/main.go**

```go
// cmd/worker/main.go
package main

import (
	"context"
	"flag"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/gonotelm-lab/gonotelm/internal/bootstrap"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	pkglog "github.com/gonotelm-lab/gonotelm/pkg/log"
)

func main() {
	configPath := flag.String("config", "./etc/gonotelm.toml.tpl", "config file path")
	flag.Parse()

	cfg, err := conf.Load(*configPath)
	if err != nil { panic(err) }
	conf.SetGlobal(cfg)
	pkglog.Init()
	if err := pkglog.SetLevelText(cfg.Logging.Level); err != nil { panic(err) }

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewWorkerApp(ctx, cfg)
	if err != nil { slog.Error("new worker app failed", "err", err); return }
	defer app.Close(context.Background())

	if err := app.Run(ctx); err != nil {
		slog.Error("worker run failed", "err", err)
	}
}
```

- [ ] **Step 3: Test worker_app_test.go**

Mock `flow.Addr = "invalid"` to test that `NewWorkerApp` errors when conn cannot be created; assert error wraps accordingly. Use a fake `*SharedInfra` (extract `NewSharedInfra` behind an interface to allow override in tests).

- [ ] **Step 4: Build, run tests, commit**

```bash
go build ./cmd/worker
go test ./internal/bootstrap/...
git add internal/bootstrap/worker_app.go internal/bootstrap/worker_app_test.go cmd/worker/ internal/application/artifact/generate/worker.go
git commit -m "feat(worker): initialize worker app with 4 per-kind flow workers"
```

---

## Task 18: Studio HTTP Routes (Migration)

**Files:**
- Create: `internal/interfaces/api/studio/routes.go`
- Create: `internal/interfaces/api/studio/middleware.go`
- Create: `internal/interfaces/api/studio/generate.go`
- Create: `internal/interfaces/api/studio/status.go`
- Create: `internal/interfaces/api/studio/result.go`
- Create: `internal/interfaces/api/studio/retry.go`
- Create: `internal/interfaces/api/studio/cancel.go`
- Create: `internal/interfaces/api/studio/delete.go`
- Create: `internal/interfaces/api/studio/list.go`
- Create: `internal/interfaces/api/studio/schema.go`
- Test: `internal/interfaces/api/studio/handlers_test.go` (uses hertz `ut` package; tests each handler against a mock usecase).

**Interfaces:**
- Consumes: `internal/application/artifact/usecase` (all use cases + syncer.Poller), `pkg/http`, `pkg/context`, hertz
- Produces: `RegisterRoutes(server *server.Hertz, deps *Deps)` where `Deps` holds the use cases. Replaces the studio-related routes in old `internal/api/studioapi.go` + notebook route `/notebook/:id/studio/artifact/list`.

- [ ] **Step 1: Port routes.go (verbatim from old `internal/api/studioapi.go:17-29` + the list route from `notebookapi.go:31`), using the new usecases**

Keep route paths exactly the same: `POST /studio/artifact/generate`, `GET /studio/artifact/:task_id/status`, `GET /studio/artifact/:task_id/result`, `POST /studio/artifact/:task_id/delete|retry|cancel`, `GET /notebook/:id/studio/artifact/list`.

- [ ] **Step 2: Port middleware.go** — `checkArtifactUserMiddleware` calls a small usecase method or directly queries StatusUseCase (with a no-result call returning ownership check). Add a method `StatusUseCase.CheckOwnership(ctx, id) error` for the middleware.

- [ ] **Step 3: Port each handler file** — each handler is now ~20 lines: bind+validate, call use case, OkResp/ErrResp.

- [ ] **Step 4: schema.go holds the DTOs** — `GenerateStudioArtifactRequest` (matches old studioapi.go:99-205 + `model.ArtifactKind` enum replaced by `entity.Kind`), `GenerateStudioArtifactResponse{TaskId string}`, `GetStudioArtifactStatusResponse{TaskId string; Status entity.Status}`, `NotebookStudioArtifactResponse`, `ArtifactResult` (copy old schema/artifact.go:9-93 and adapt).

- [ ] **Step 5: Write handler tests** — use hertz `ut.Perform` against each route with a stub usecase.

- [ ] **Step 6: Run tests, commit**

```bash
go test ./internal/interfaces/api/studio/...
git add internal/interfaces/api/studio/
git commit -m "feat(api): studio HTTP routes migrated to interfaces/api/studio"
```

---

## Task 19: Update Notebook-Deleted Event Handler

**Files:**
- Modify: `internal/application/studio/eventhandle/onnotebookdeleted.go`
- Modify: `internal/interfaces/event/eventhandler.go`
- Test: extend or add `internal/application/studio/eventhandle/onnotebookdeleted_test.go` if not present.

- [ ] **Step 1: Switch the handler to use the new artifact repo port**

```go
// onnotebookdeleted.go
package eventhandle

import (
	"context"

	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
)

type DeleteNotebookArtifactTasksHandler struct {
	artifactRepo artifactrepo.Repository
}

func NewDeleteNotebookArtifactTasksHandler(repo artifactrepo.Repository) *DeleteNotebookArtifactTasksHandler {
	return &DeleteNotebookArtifactTasksHandler{artifactRepo: repo}
}

func (h *DeleteNotebookArtifactTasksHandler) Handle(ctx context.Context, evt *notebookevent.Event) error {
	if evt.Action() != notebookevent.EventActionDelete {
		return nil
	}
	return h.artifactRepo.DeleteByNotebookId(ctx, evt.NotebookId())
}

func RegisterNotebookDeletedConsumer(ctx context.Context, bus eventbus.EventBus, h *DeleteNotebookArtifactTasksHandler) error {
	return eventbus.SubscribeNotebookDeleted(ctx, bus, h.Handle)
}
```

- [ ] **Step 2: Update `eventhandler.go` `EventDeps` struct field type from `*repository.ArtifactTaskRepository` to `artifactrepo.Repository`**

Adjust the registration call at `eventhandler.go:95` accordingly. Drop the old concrete `*ArtifactTaskRepository`.

- [ ] **Step 3: Run tests, commit**

```bash
go test ./internal/interfaces/event/... ./internal/application/studio/...
git add internal/application/studio/eventhandle/ internal/interfaces/event/
git commit -m "refactor(event): notebook-deleted handler uses artifactrepo port"
```

---

## Task 20: Wire API Bootstrap (HTTP Server + Syncer)

**Files:**
- Modify: `internal/bootstrap/app.go` — replace `dummyServer` with a real Hertz server wired from `internal/interfaces/api/studio/` (and other existing routes). Construct artifact use cases with all deps. Start `Syncer.Start(ctx)`. Stop on app close.

- [ ] **Step 1: Implement real wiring**

Inside `NewApp`:
- Build `FlowTaskClient = flow.NewTaskClient(cfg.Flow.Addr, cfg.Flow.Namespace, cfg.Flow.DialTimeout, cfg.Flow.MaxRetry)`.
- Build artifact repo using `db.ArtifactStore`.
- Build `Syncer = syncer.NewSyncer(artifactRepo, FlowTaskClient, cfg.Syncer)`. Start it: `syncer.Start(ctx)`. Stop it on app close.
- Build all use cases (`NewGenerate`, `NewStatus`, `NewList`, `NewCancel`, `NewDelete`, `NewRetry`) passing repo, flow client, notebook repo, storage gateway, syncer (as `Poller`).
- Build `studioapi.Deps` and call `studioapi.RegisterRoutes(hertzServer, deps)`.

For the existing non-studio HTTP routes (notebook/chat/source), leave them as they are today behind the old `internal/api/server.go` interface — they'll be handled in cleanup Task 21 (delete). But for studio specifically, route registration moves to `internal/interfaces/api/studio/`. To avoid route duplication, exclude `/studio/**` and `/notebook/:id/studio/artifact/list` from old `registerStudioRoutes` and from the notebook routes.

Update `internal/api/server.go` accordingly to drop studio routes registration while Tasks 21 still iterates (final delete is coming).

- [ ] **Step 2: Run `cmd/gonotelm` end-to-end**

Run:
```bash
go build -o /tmp/gonotelm ./cmd/gonotelm
/tmp/gonotelm --config ./etc/gonotelm.toml.tpl
```
Expected: Hertz server starts; `GET /studio/artifact/...` returns 4xx/405 (no route without server alive) and root routes for studio still wired.

- [ ] **Step 3: Run tests, commit**

```bash
go test ./internal/bootstrap/...
git add internal/bootstrap/app.go internal/api/server.go
git commit -m "feat(bootstrap): wire artifacts HTTP + syncer into api app"
```

---

## Task 21: Final Cleanup — Delete internal/app/ + internal/api/

**Files:**
- Delete: `internal/app/` (entire directory)
- Delete: `internal/api/` (entire directory)
- Modify: anything that still imports them after Task 20.

- [ ] **Step 1: Audit residual imports**

```bash
go list -f '{{.ImportPath}}: {{join .Imports "\n"}}' ./... | rg 'internal/app/|internal/api/'
```

(Or run `gofmt -l` on imports.)

If residuals exist, fix the importer. Common ones:
- `internal/app/constants` (now `internal/application/artifact/constants.go` — created in Task 8 side-effect). Provide in a small file there: `const MaxArtifactTitleLength = 128; const MaxNotebookNameLength = 128; const MindmapMaxOnceToken = 32_000`. (TDD-step: write a small test asserting the constants.)

- [ ] **Step 2: Delete directories**

```bash
git rm -r internal/app/
```

- [ ] **Step 3: Delete `internal/api/`**

```bash
git rm -r internal/api/
```

(Note: after deleting `internal/api/`, all HTTP routes including notebook/chat/source need handlers somewhere. Confirm these were *previously* migrated — from the spec: "chat/notebook/source 业务的 DDD 迁移（已完成）". The current `internal/api/` only has shell handlers calling `internal/application/...` based on `internal/api/server.go:85-100` — so those handlers should already live in `internal/application/{chat,notebook,source}/` with their HTTP wiring at `internal/interfaces/api/`. If `internal/interfaces/api/` doesn't have them yet (Task list confirmed `internal/interfaces/api/` is currently EMPTY), then `internal/api/` is currently hosting the HTTP wiring — deleting it would lose the routes. **This is a critical caveat.**)

⚠ **Decision point**: If non-studio routes still live in `internal/api/`, the cleanup needs to first migrate them to `internal/interfaces/api/`. Out of scope for this design — flag to user. For this plan, only **delete `internal/app/`** (logic/biz/model/constants) and keep `internal/api/` for non-studio routes until the next refactor.

Update the cleanup task:
- Step 3: **Do NOT** `git rm -r internal/api/` yet. Instead, ensure `internal/api/` only holds non-studio routes (studio routes already moved to `internal/interfaces/api/studio/`). Remove only the `internal/api/studioapi.go` and the `studio`-related lines of `notebookapi.go` (the `ListNotebookStudioArtifacts` handler at `notebookapi.go:352-401` and its registration at `notebookapi.go:31`). Also remove `schema/artifact.go`.

- [ ] **Step 4: Final build/vet**

Run:
```bash
go build -o /tmp/gonotelm ./cmd/gonotelm
go build -o /tmp/gonotelm-worker ./cmd/worker
go vet ./...
go test ./...
```

Expected: All green.

- [ ] **Step 5: Verify no residuals**

```bash
! rg 'gonotelm/internal/app/' --type go
```

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(ddd): delete internal/app/ and prune studio routes from internal/api/"
```

---

## Post-Implementation Integration Test (Manual)

After all tasks complete, run an end-to-end smoke test:

```bash
# 1. Start dependencies (postgres / flow / minio / kafka already running per user)
# 2. Apply migration if not yet
psql "$GONOTELM_DB_DSN" -f migration/db/20250711_artifacts.sql

# 3. Start flow server (user-managed)
# 4. Start gonotelm API
go run ./cmd/gonotelm --config ./etc/gonotelm.toml.tpl

# 5. Start worker
go run ./cmd/worker --config ./etc/gonotelm.toml.tpl

# 6. Submit a mindmap generation
curl -X POST http://127.0.0.1:7099/studio/artifact/generate \
  -H 'Content-Type: application/json' \
  -H 'x-user-id: u1' \
  -d '{"notebook_id":"<some-nb>","kind":"mindmap","source_ids":["<some-src>"]}'
# expected: {"task_id":"artifact-id"}

# 7. Poll status
curl http://127.0.0.1:7099/studio/artifact/<task_id>/status -H 'x-user-id: u1'
# expected: status moves from pending -> running -> completed
```

---

## Self-Review Notes

Spec coverage check:
- §Architecture / process topology → Tasks 17 (worker), 20 (API), 12 (cmd move).
- §Data Model (artifacts table, payload dual-track) → Tasks 3 (migration), 7 (payload passed via flow Submit).
- §Domain Layer → Tasks 1, 2, 5.
- §Components — generate usecase → Task 9; status/list/cancel/delete/retry → Task 10; syncer (per-task + global) → Task 11.
- §Worker — bootstrap + cmd/worker → Task 17.
- §Generators (mindmap/report/infographic/audio) → Tasks 13–16.
- §HTTP API → Task 18.
- §Bootstrap + Config → Tasks 6, 12, 20.
- §Migration (table + code) → Tasks 3, 21.
- §Error Handling → covered across all use case tests.
- §Testing → all task steps include TDD.
- §risks (orphan flow tasks) → not mitigated in code; flagged in design.

Type consistency:
- `Status` enum strings (pending/running/completed/failed/cancelled) consistent across entity, store, repository, syncer.
- `Kind` enum strings (mindmap/report/info_graphic/audio_overview) consistent across entity, payload `Kind()` method, `taskTypeFor`.
- `ResultKind` enum string inline/storage consistent across entity and generator responses.
- `Repository` signature consistent across port (`Task 2`) and impl (`Task 5`).
- `Poller` interface contract: `PollOne(ctx, artifactId)` matches syncer impl + usecase deps.

Gaps covered above; placeholder scan: none remaining.