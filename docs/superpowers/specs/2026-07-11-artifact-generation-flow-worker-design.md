# Artifact Generation via Flow + Worker (DDD)

## Summary

把 studio 的产物生成（artifact generation）从当前 in-process 任务循环迁移为：flow 服务做任务调度，独立的 worker 进程执行生成逻辑。API 进程向 flow 提交任务，worker 从 flow 拉取并执行，结果经 flow 回流，API 进程后台协程同步状态到 gonotelm 本表。

完成 DDD 重构最后一个业务领域的建模，并引入两个新入口：`cmd/worker`（worker 进程）和 `internal/domain/artifact`（领域层）。

## Background

### 现状

- `internal/app/logic/studio/` 4 个 generator（mindmap、report、info_graphic、audio_overview）在一个进程内跑：`taskLoop` 轮询 Postgres `artifact_tasks` 表 `TryClaimTask`，提交到 `ants.MultiPool` 同步执行。`ArtifactKind`、`ArtifactStatus`、`ArtifactTask` 模型在 `internal/app/model/artifact.go`。
- `artifact_tasks` 表自带完整状态机：pending→running→completed/failed/cancelled/expired，含 `RunId`、`LockNo`、`expiredAt`，行级 claim via `FOR UPDATE SKIP LOCKED`。
- DDD 重构中期：`internal/domain/artifact/{entity,errors,repository}` 是**空目录**。`internal/application/studio/` 只有 `eventhandle/onnotebookdeleted.go`。`internal/application/chat/agent/agent.go` 已有新范式：`pkg/agent.Agent[State]` + domain 层 `agentize.Service` 工具，是 worker 复用 agent 的参考。
- `cmd/main.go` 是唯一入口，加载 conf + `bootstrap.NewApp`（当前 HTTP 服务器是 `dummyServer{}`）。

### flow 服务能力（已建，独立部署）

- `task.Client.Submit(namespace, taskType, payload, WithMaxRetry(n)) → Task{id, state=INITED}`。
- `worker.Client.Register(namespace, taskType) → workerId`，`Start()` 启动 Poll + Heartbeat 循环。
- worker 处理完调 `Report(SUCCESS|FAIL, payload)` → flow 把 result/error 存进 `tasks.result`/`tasks.error`，CAS 切 DONE/FAILED。
- `task.Client.Get(taskId) → Task` 可查状态与结果。
- `task.Client.Cancel(taskId)` 标记 CANCELLED，下个 heartbeat 推回 worker 中断 handler。
- RetryMender 自动重试 FAILED 任务（指数退避，上限 max_retry）。
- StaleDetector 把心跳超时的 RUNNING 任务标为 FAILED。
- TaskState: INITED / RUNNING / DONE / FAILED / CANCELLED。

## Architecture

### 进程拓扑

```
┌──────────────────────── gonotelm-lab/gonotelm (单一代码库) ─────────────────────┐
│                                                                                 │
│  ┌───────────────────┐         gRPC         ┌────────────────────────────────┐ │
│  │  API 进程          │ ───────────────────▶ │ flow 服务 (独立部署)           │ │
│  │  cmd/gonotelm/     │   task.Submit/Get/   │  task 库 + worker 队列         │ │
│  │                    │   Cancel             │  RetryMender + StaleDetector  │ │
│  │  - HTTP server     │ ◀───── task.Get ──── │  Heartbeat 推 cancel          │ │
│  │  - artifact repo   │                      └──────────┬─────────────────────┘ │
│  │    (postgres)      │                                 │ worker.Poll/Report    │
│  │  - flow.TaskClient │                                 │ Heartbeat             │
│  │  - syncer 协程     │                                 ▼                       │
│  │  - agentize.Svc    │ (元数据校验、不跑 agent)  ┌────────────────────────────┐ │
│  └───────────────────┘                          │  Worker 进程              │ │
│                                                 │  cmd/worker/              │ │
│                                                 │   - 4 个 worker.Client    │ │
│                                                 │     (per-kind task_type)  │ │
│                                                 │   - pkg/agent.Agent[State]│ │
│                                                 │   - agentize-backed 工具  │ │
│                                                 │   - LLM / storage gateway │ │
│                                                 │   - 无 DB / 无 HTTP       │ │
│                                                 └────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 职责切分

| 维度 | API 进程 `cmd/gonotelm` | Worker 进程 `cmd/worker` |
|---|---|---|
| 触发 | HTTP 请求 | flow.Poll 长轮询 |
| DB 访问 | 有（postgres + artifact repository） | **无** |
| HTTP 端口 | 暴露 | 无 |
| flow 角色 | `task.Client`（Submit/Get/Cancel） | `worker.Client` x4（Register/Poll/Heartbeat/Report） |
| LLM/storage | 不调用（仅元数据校验） | LLM gateway、MinIO storage |
| agentize.Service | 元数据/源归属校验 | source 检索 + 工具绑定 |
| `pkg/agent` | 不用 | ReAct agent 执行 |
| artifact 表 | 读写（同步器独占写 status） | 不接触 |

### task_type 粒度

每 kind 一个 task_type：`artifact.mindmap` / `artifact.report` / `artifact.info_graphic` / `artifact.audio_overview`。

worker 进程用单 gRPC conn 拉起 4 个 `worker.Client`，各自 Register 不同 task_type，独立配置 max_retry，flow 层可按 kind 看 worker 数与任务量。

### 对外 ID 策略

- `artifact.id`（UUID，gonotelm 自生成）是对外的 `task_id`，URL `/studio/artifact/:task_id/*` 沿用。
- `flowTaskId` 隐式存在于 artifacts 表内部字段；外部不直接接触。

## Data Model

### artifacts 表（重构后）

```sql
CREATE TABLE artifacts (
  id            UUID        PRIMARY KEY,                 -- artifact.id，对外 ID
  notebook_id   UUID        NOT NULL,
  user_id       ...         NOT NULL,
  kind          VARCHAR(32) NOT NULL,                   -- mindmap/report/info_graphic/audio_overview
  status        VARCHAR(16) NOT NULL,                   -- pending/running/completed/failed/cancelled
  flow_task_id  VARCHAR(64) NOT NULL,                   -- flow 生成的 UUID；retry 时被更新
  title         VARCHAR(...) NULL,                      -- 生成完成后由同步器回填
  result        BYTEA       NULL,                        -- 同步器从 flow.task.result 取回
  result_kind   VARCHAR(16) NULL,                       -- inline/storage
  payload       JSONB       NOT NULL,                   -- kind-specific 输入快照
  created_at    TIMESTAMPTZ NOT NULL,
  updated_at    TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_artifacts_notebook ON artifacts(notebook_id);
CREATE INDEX idx_artifacts_status  ON artifacts(status);   -- 同步器扫 pending/running
UNIQUE (flow_task_id);                                       -- 旧 flowTaskId 仍可能存于 flow，见 retry 风险
```

**移除的字段**（相比 `artifact_tasks`）：`RunId`（worker 不与 DB 抢锁）、`LockNo`、`expiredAt`（flow 的 StaleDetector + RetryMender 接管）、`expired` 状态（flow 无等价物）。**不保留 error 列**——失败信息只在 `flow.task.error`，前端调 status 时回源 flow。

### status 与 flow TaskState 映射

| flow TaskState | artifact status |
|---|---|
| INITED | pending |
| RUNNING | running |
| DONE | completed |
| FAILED | failed |
| CANCELLED | cancelled |

flow 的 StaleDetector 把心跳超时的 RUNNING 任务切到 FAILED，RetryMender 按指数退避重新排队——这等于旧 `expiredAt` 的能力，且无需 gonotelm 介入。

### Payload 双轨

- `artifacts.payload` (JSONB)：提交时存的 kind-specific 输入快照（sourceIds / orientation / detailLevel / audioStyle 等），供 retry 时复用与调试。
- `flow.task.payload` (bytes, JSONCodec)：submit 时由 API 进程序列化同一份 kind-specific 输入传入 flow。worker `RegisterTyped[generateInput, generateOutput]` 反序列化。

两者字段一致，只在存储介质分两份，避免 worker 反向依赖 DB。

## Domain Layer

`internal/domain/artifact/`（目前空目录）将填充：

- `entity/artifact.go` — `Artifact` 聚合根：`Id / NotebookId / UserId / Kind / Status / FlowTaskId / Title / Result / ResultKind / Payload / CreatedAt / UpdatedAt`。行为方法：
  - `NewArtifact(notebookId, userId, kind, payload)` → status=pending
  - `BindFlowTaskId(flowTaskId)` — submit 后绑定
  - `MarkFromFlow(state, result, title)` — 同步器回写时用
  - `MarkRetrying(newFlowTaskId)` — retry 时重置 status=pending、清空 title/result、绑定 newFlowTaskId
  - `MarkCancelled()` — cancel 时直接置 cancelled
  - `IsTerminal()` 谓词
- `entity/payload.go` — `Kind` 枚举 + `Payload` 接口 + 各 kind 具体 struct（`MindmapPayload`、`ReportPayload`、`InfoGraphicPayload`、`AudioOverviewPayload`），含 sourceIds、orientation、detailLevel、audioStyle 等字段。
- `errors/errors.go` — `ErrArtifactNotFound`、`ErrNotOwner`、`ErrCannotCancelInState`、`ErrCannotRetryInState` 等。
- `repository/repository.go` — Repository 端口接口：`Create`、`Get`、`GetByFlowTaskId`、`ListByNotebook`、`UpdateStatus`、`Delete`、`ListByStatus`（同步器扫用）等。

Repository 实现放 `internal/infrastructure/repository/artifact.go`（复用现有 mapper 模式），postgres store schema 加 `internal/infrastructure/database/schema/artifact.go` 替代 `artifacttask.go`。

## Components

### 1. API 进程：application/artifact/

```
internal/application/artifact/
├── generate.go          // generate usecase
├── status.go            // status / list usecase
├── retry.go             // retry usecase
├── cancel.go            // cancel usecase
├── delete.go            // delete usecase
└── syncer/
    ├── per-task.go      // PollOne(artifactId) 协程
    └── global.go        // 兜底扫描器协程
```

#### generate usecase

```
1. artifact = entity.NewArtifact(notebookId, userId, kind, payload)   // id 进程内生成, status=pending
2. flowTaskId = flow.Submit(kindTaskType(kind), payload)              // 远程调用，先做
3. artifact.BindFlowTaskId(flowTaskId)
4. repo.Create(artifact)                                               // 本地事务
5. go syncer.PollOne(artifact.Id)                                      // per-task 即时启动
6. return artifact.Id
```

**提交顺序**：先 flow.Submit 后 repo.Create。

理由：flow.Submit 失败 → HTTP 直接 500，artifact 表不写入，无脏数据。反过来 submit 失败要回滚事务或留孤儿 pending 行，多一条失败路径。

**孤儿风险**：step 2 成功、step 4 commit 前进程崩溃会留下孤儿 flow task。采取轻量兜底——接受极小概率，flow 侧未来可加 INITED-TTL 扫描清理，gonotelm 不引入分布式幂等。同步器只查 artifacts 表里 pending/running 行调 flow.Get，孤儿不会影响 list/status 路径。

#### status usecase

```
1. artifact := repo.Get(id)
2. 权限校验 artifact.UserId == ctx.userId
3. if artifact.IsTerminal() → return artifact.Status (+ result 如果 completed)
4. else → flowTask = flow.Get(artifact.FlowTaskId)
         return mappedStatus(flowTask.State) (+ flowTask.Error 详情显示 LLM/storage 失败原因)
```

#### list usecase

```
1. artifacts := repo.ListByNotebook(notebookId)
2. 返回 artifact 列表（终态直接出表，非终态用同步器上次写入的表 status）
3. list 不调 flow
```

**Trade-off**：list 可能状态滞后几秒，前端想要最新调 `/status/:id`。配合 per-task 协程（提交后几秒内同步器已写入表）实际体感够用。

#### retry usecase

```
1. artifact := repo.Get(id)
2. if artifact.Status != failed && != cancelled → ErrCannotRetryInState
3. oldFlowTaskId := artifact.FlowTaskId
4. flowTaskId' = flow.Submit(kindTaskType, artifact.Payload)        // 新的 flow task
5. 事务{ artifact.MarkRetrying(flowTaskId')                          // status=pending, flow_task_id=flowTaskId'
          title/result/result_kind 清空 }
6. flow.Cancel(oldFlowTaskId)                                        // 防旧 task 被 worker 重新拉起（已终态则 no-op）
7. go syncer.PollOne(artifact.Id)
8. return artifact.Id                                              // 不变
```

**复用 artifact，新 flowTaskId**：artifact.id 沿用，不影响 URL。step 6 防止旧 task 重跑：旧 flowTaskId 若已终态则 `flow.Cancel` no-op，若仍 INITED/RUNNING 则被标 CANCELLED，worker 在下次 heartbeat 时收到 cancel 推送就退出对应 handler。极端场景下旧 worker 已开始执行且还没遇到心跳查 cancel —— 会浪费一次算力但不会污染 artifacts 表（同步器只查 artifact.FlowTaskId 当前的值，已切到新 id）。

#### delete usecase

```
1. artifact := repo.Get(id)
2. if !artifact.IsTerminal() → flow.Cancel(artifact.FlowTaskId)
3. if artifact.ResultKind == storage → minio 删除关联的存储对象
4. repo.Delete(id)
5. 关闭该 artifact 的 per-task 协程（如有）
```

#### cancel usecase

```
1. artifact := repo.Get(id)
2. if artifact.IsTerminal() → ErrCannotCancelInState
3. flow.Cancel(artifact.FlowTaskId)
4. 状态由同步器在下次轮询时同步为 cancelled（或这里同步调 repo.UpdateStatus(cancelled)）
```

### 2. 状态同步器（双轨）

`internal/application/artifact/syncer/`

**Per-task 协程**：每次 generate / retry 在 API 进程内 spawn 一个 `goroutine PollOne(artifactId)`：
- 每 2s（配置：`syncer.per-task-interval`，默认 2s）调 `flow.Get(artifact.FlowTaskId)`。
- state 变化 → `repo.UpdateStatus(artifact.Id, mappedStatus, result, title)`。
- 终态后协程退出。

**全局兜底协程**：API 进程启动时 spawn，处理 per-task 协程因进程重启而遗漏的 artifact：
- 周期（`syncer.global-interval`，默认 5s）`repo.ListByStatus(['pending', 'running'])` 分页扫表（每页 N 行，避免一次扫爆）。
- 对每行调 `flow.Get`，状态变化则 `UpdateStatus`。
- 与 per-task 重叠时 repository 的 `UpdateStatus` 用状态机 CAS（只允许向终态单调推进），避免并发覆盖写。

**单实例足够**：与 HTTP server 同进程。未来 API 进程水平扩展时，全局扫描可以切到 `FOR UPDATE SKIP LOCKED`，与 flow 同款流派。

### 3. Worker 进程：cmd/worker/main.go

```
cmd/worker/
└── main.go
    1. 加载 etc/gonotelm.toml（worker section）
    2. bootstrap.NewWorkerApp(config)
       ├── Infra：LLM gateway / storage gateway / agentize.Service
       │   (与 API 进程共享 bootstrap.SharedInfra)
       ├── flow worker conn (单 grpc.ClientConn)
       ├── 4 个 generator handler (per-kind)
       ├── 4 个 flowworker.Client (共享 conn, 各 Register 不同 task_type)
       └── 每个 Client.Handle(handler) 注册 typed handler
    3. 对每个 Client 调 Start() (并发 poll/heartbeat)
    4. 阻塞 ctx，signal 优雅停
```

worker 不依赖 `database/postgres`、`cache/redis`、`mq/kafka`、`eventbus`、`interfaces/api` 等包，构建通过 `go build -o gonotelm-worker ./cmd/worker`。

### 4. Generator 范式

`internal/application/artifact/generate/`：

```
generate/
├── generate.go        // 公共 RunRequest/RunResponse/SessionState + 工具绑定
├── mindmap.go
├── report.go
├── infographic.go
└── audiooverview.go
```

参照 `internal/application/chat/agent/agent.go` 范式：
- 每个 generator 是一个函数 `Generate(ctx, *Request) (*Response, error)`
- 内部构造 `pkg/agent.Agent[*SessionState]`，绑定 agentize 驱动的 source 工具（`readsource`/`grepsource`/`querysource`/`statsource`，取 `internal/application/chat/agent/tools/` 路径，依赖 `agentize.Service` 而非旧 `bizsource.AgentBiz`）。
- prompt 从旧 `internal/app/biz/prompt/studiomindmap.go` 等模板迁移到 `internal/application/artifact/prompt/`，脱离 biz 依赖。

**generate handler 接口**：

```go
// 在 worker 进程内用 RegisterTyped[generateInput, generateOutput] 注册
type GenerateRequest struct {
    ArtifactId  string
    NotebookId  string
    UserId      string
    SourceIds   []string
    Kind        Kind
    Payload     Payload         // kind-specific
}

type GenerateResponse struct {
    Title      string
    Result     []byte           // 最后序列化为 OkResult.Data
    ResultKind ResultKind       // inline / storage
}
```

错误时构造错误 string 作 `ErrorResult.Data`（artifact 表不存 error，失败详情只在 flow.task.error，前端查 status 时走 flow.Get 取回）。

## Bootstrap 与配置

### 命令布局

```
cmd/
├── gonotelm/           // 新增子目录
│   └── main.go         // 移自 cmd/main.go；加载 conf + bootstrap.NewApp + 起 HTTP/syncer
└── worker/             // 新增
    └── main.go         // 加载 conf + bootstrap.NewWorkerApp + 启动 4 个 worker.Client
```

**双 main 共用 conf**：`etc/gonotelm.toml` 既有 `[server]`（API）也有 `[worker]`（worker）两 section，两个 main 各按模式读对应 section。`bootstrap.SharedInfra(config) → Infra` 抽出 LLM/storage/agentize 共享装配代码。

### 配置新增

`internal/conf/` 增加：

```toml
[flow]
addr        = "flow.example:9443"          # flow gRPC 地址
namespace   = "gonotelm"
max-retry   = 3                             # 默认 Submit max_retry
dial-timeout = "5s"

[syncer]
per-task-interval = "2s"                    # per-task 协程轮询间隔
global-interval    = "5s"                    # 全局扫描协程周期
global-batch-size  = 100                    # 每周期扫描的最大行数

[worker]
name             = "gonotelm-worker-1"
max-concurrency  = 4                         # 单 worker.Client 的 MaxConcurrency
heartbeat        = "5s"
task-types       = ["artifact.mindmap", "artifact.report", "artifact.info_graphic", "artifact.audio_overview"]
```

## HTTP API

新路由放在 `internal/interfaces/api/`（旧 `internal/api/studioapi.go` 迁移）：

| Method | Path | 说明 |
|---|---|---|
| POST | `/studio/artifact/generate` | 提交生成任务，返回 artifact.id |
| GET | `/studio/artifact/:task_id/status` | 查任务状态+结果（终态出表，非终态回源 flow） |
| GET | `/notebook/:id/studio/artifact/list` | 列 artifact 元数据（不调 flow） |
| POST | `/studio/artifact/:task_id/retry` | 复用 artifact，新建 flow task |
| DELETE | `/studio/artifact/:task_id` | 删除（含非终态先 cancel、storage 类清 minio） |
| POST | `/studio/artifact/:task_id/cancel` | 取消（调 flow.Cancel） |

## Migration

### 表迁移

- 废弃 `artifact_tasks` 表（保留旧迁移记录），新建 `artifacts` 表迁移脚本 `migration/db/xxx_artifacts.sql`。
- 存量数据迁移：将 `artifact_tasks` 终态行映射到 `artifacts` 行（对齐字段、丢弃 `RunId`/`LockNo`/`expiredAt`）。存量的 pending/running 行无法映射 flowTaskId，提交期允许直接清表（产品同意低风险）。

### 代码迁移

| 旧路径 | 处理 |
|---|---|
| `internal/app/logic/studio/` | 4 generator 逻辑参考迁移到 `internal/application/artifact/generate/`，删除旧包 |
| `internal/app/agent/tool/` | 不复用，改用 `internal/application/chat/agent/tools/`（agentize-backed） |
| `internal/app/biz/prompt/studio*.go` | 抽到 `internal/application/artifact/prompt/` |
| `internal/app/biz/artifact/` | 删除，业务走 domain repo |
| `internal/app/model/artifact.go` | 删除，entity 在 `internal/domain/artifact/entity/` 重建 |
| `internal/api/studioapi.go`、`notebookapi.go` 中的 studio 路由 | 迁到 `internal/interfaces/api/studio/`（新建） |
| `internal/application/studio/eventhandle/onnotebookdeleted.go` | 保留，改用新 artifact repository |
| `internal/interfaces/event/eventhandler.go` | 相关注册保留 |

### DDD 重构收尾

本工作完成后，studio 业务从旧 `internal/app/{biz,logic,model}` 完整迁出，DDD 重构剩余的其他业务（source、chat、notebook）已在前期完成，本工作即重构最后一公里。

## Error Handling

| 场景 | 处理 |
|---|---|
| flow.Submit 失败 | HTTP 500，无表写入，客户端重试 |
| worker handler panic | flow.Recover 上报到 FAIL，RetryMender 自动重试 |
| worker 心跳超时 | flow StaleDetector 把 task 标 FAILED，RetryMender 自动重试 |
| API 进程提交时崩溃留下孤儿 INITED task | flow 侧 GC（未来加 INITED-TTL 扫描），当前接受极小概率 |
| 同步器 flow.Get 超时 | 跳过该行，下周期重试，不写表 |
| retry 时旧 flowTaskId 仍 RUNNING | API 进程在 retry 事务后调 `flow.Cancel(旧 flowTaskId)` 防止被 worker 拉起 |
| delete 时非终态 | 先 `flow.Cancel` 后 `repo.Delete` |
| minio 删除 storage 类 result 失败 | 不阻塞 delete 操作，记日志 |

## Risks

- **孤儿 INITED task**：极小概率，flow 侧未来加 INITED-TTL 扫描清理。短期可手动或定时清理脚本。
- **retry 时旧 flowTaskId**：旧 task 在 flow 侧若已终态自然留作历史；若是 INITED/RUNNING，需 API 侧 cancel。漏 cancel 会出现旧 worker 跑出旧 result 但被新 flowTaskId 同步器忽略，不会污染表（同步器只查 artifact.FlowTaskId，即新 id），但浪费一次 worker 算力。
- **flow 大体量 result 存储**：report 文本可能 KB-MB，flow.task.result 为 bytea 字段存全量。短期可接受，长期若成为热点考虑 result 存 minio 后 URL 写 flow（require worker 访问 minio + DB URL 列，worker 退一步碰 storage 即可，不碰 DB）。
- **list 状态滞后**：非终态 artifact 的 list 返回表内 status，可能滞后几秒。前端要最新状态调 /status。

## Testing

- domain entity：表驱动测状态转换与方法不变式。
- repository：测试套件复用现有 postgres store 测试模式（`artifacttaskstore_test.go`）。
- generate usecase：mock flow.TaskClient，验证 Submit/Get 调用序列与孤儿路径。
- syncer：mock flow + repo，验证 per-task 与 global 双轨协同、CAS 状态单调。
- worker handler：mock pkg/agent + agentize，验证 GenerateRequest → GenerateResponse 翻译。
- 集成测试：mock flow server（用 flow/client/example 的 echo worker 或自建 in-memory server），end-to-end 验证 submit→poll→report→sync→status 路径。

## Scope & Non-Goals

- 本工作覆盖：artifact domain 建模、application use cases、HTTP 路由迁移、worker 进程与 generator、状态同步器、artifacts 表迁移。
- 不覆盖：flow 服务自身的改动（.flow 不动）；chat/notebook/source 业务的 DDD 迁移（已完成）；agentize.Service 的扩展（按需小改）；旧 studio generator 逻辑的算法重写（仅平台迁移）。
- 实施完后旧 `internal/app/logic/studio/`、`internal/app/biz/artifact/`、`internal/app/model/artifact.go` 删除，DDD 重构收尾。