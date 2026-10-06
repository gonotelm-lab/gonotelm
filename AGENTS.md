# AGENTS.md

GoNoteLM backend: ingests PDFs, links, and notes into a queryable knowledge base, then generates artifacts — mindmap, report, quiz, flashcard, slides, infographic, audio/video overview.

Go with a clean/layered architecture. Hertz (HTTP), PostgreSQL, Redis, Milvus (vector), ClickHouse (OLAP), MinIO (object storage), Kafka (MQ), flow (gRPC long-task queue), OpenSandbox (code execution). The web UI lives in a separate repo, `gonotelm-lab/gonotelm-web`.

## Dependency rules (hard constraints)

Dependencies point inward, toward `core`:

```
cmd → bootstrap → interfaces → application → domain → core
                        ↘ infrastructure ↗        pkg (standalone; internal/ never imports it)
```

- `internal/core` imports no other internal package.
- `internal/domain/<domain>` imports only `core` and `pkg` — never `infrastructure`, `application`, or `interfaces`.
- `pkg/` never imports `internal/`.
- Infrastructure appears only in `application`, `interfaces`, and `bootstrap`.

## Layout

```
cmd/                      4 binaries; main only does: parse -config → load conf → init log → bootstrap → Run/Close
  notelm/                 HTTP API, :7099 by default
  sourcejob/              consumes source.preparation (MQ)
  worker/                 consumes artifact.* flow tasks
  initjob/                one-shot business-data init; exits non-zero on failure or interrupt, safe to retry
  migrate/                goose schema migration (deliberately separate from initjob)

internal/
  core/                   innermost layer, zero internal dependencies
    entity/               entity base Base: Id / times / deleted / buffered domain events (AddEvent, PullEvents)
    valobj/               value objects: Id, Uid, Time, StoreKey
    event/                Event interface and Category (inprocess | interprocess)
    adapter/              ports to outside capabilities: ObjectStore, Summarizer, TitleMaker, ImageInterpreter, DistributedLock
  domain/<domain>/        pure business logic; domains = artifact, chat, identity, notebook, sandbox, source, worker
    entity/ vo/           aggregates and value objects; constructors validate (NewNotebook returns error), mutations call AddEvent
    repository/           interfaces and ListSpec only — implementations always live in infrastructure
    service/              logic spanning entities/aggregates (e.g. source parse → chunk → index)
    errors/ event/        domain errors and domain events
  application/            use-case orchestration, one file per use case
    notelm/<domain>/      <usecase>.go: NewXxxHandler + XxxHandleCommand + Handle(ctx, cmd)
                          basehandler.go holds the load-and-authorize step shared by the domain's handlers
                          eventhandle/ subscribes to in-process events; suggestion/, syncer/ are background goroutines
    sourcejob/source/     PrepareSourceHandler, consuming MQ
    worker/artifact/      per-artifact generation pipelines (agent + step + prompt + validation)
    initjob/              init task registration
    shared/contract/      notelm ↔ worker cross-process wire contract (WorkerInput / WorkerOutput)
  infrastructure/         port implementations and external components
    repository/           implements domain repository interfaces: store + mapper (persistence model ↔ entity)
    database/             GORM stores and schema/ (Postgres table models)
    cache/ olap/ vectordb/ storage/ mq/    Redis / ClickHouse / Milvus / MinIO / Kafka
    llm/                  chat, embedding, rerank, text2audio, text2image clients
    sandbox/              OpenSandbox execution environment (used by slides / video generation)
    flow/                 long-task queue client (Submit / Get / Cancel)
    adapter/              implementations of the core/adapter ports (object storage, summarizer, title, image interpretation, distributed lock)
    eventbus/             routes by event.Category(): inprocess dispatches locally, interprocess goes to MQ; CompositeEventBus is the single publish entry point
  interfaces/             delivery layer
    api/notelm/           Hertz routes and middleware; schema/ holds request/response DTOs
    entrypoint/worker/    decodes a flow task into contract.WorkerInput and calls into application
    event/                per-process event handler registration (notelm in-process, sourcejob over MQ)
  conf/                   one config struct per binary, loading etc/<binary>.toml.tpl with env substitution; shared/ holds the common InfraConfig
  bootstrap/              composition root: New<Binary> wires infra → repository → service → handler → interfaces in order
    shared/                Infra component registry; NewInfraWith builds the requested Components and closes them together

pkg/                      business-agnostic reusable libraries: agent, llm, pipeline, batch, errors, trace, http, testsuite…
migration/                goose(SQL) / clickhouse / milvus / msgqueue / storage DDL, shared with unit tests
etc/                      per-binary config templates (.toml.tpl); values come from .env (GONOTELM_ prefix)
deploy/dev/               local middleware docker-compose and Makefile
docs/superpowers/         design docs: specs/ for designs, plans/ for implementation plans
tests/deptest/            cross-dependency integration smoke tests
assets/                   slides templates, opensandbox Dockerfile, node build scripts
```

## HTTP API layer (`internal/interfaces/api/notelm`)

- **Routing**: `server.go` is the only place that builds the server and its groups. `registerRoutes` mounts `/api/v1` (CSRF) → `v1AuthedGroup` (`authMiddleware`) → one `register<Domain>Routes(g *route.RouterGroup)` per file. Public routes are registered against `v1Group` instead (only `/api/v1/auth` today). Adding an endpoint = a handler method on `*Server` + one line in a `register*Routes`. A static sibling wins over a `:param` (`/artifacts/style-previews` vs `/artifacts/:id`), so both can coexist.
- **Middleware**: global chain is `Tracing → Recovery → CORS → Logging`; CSRF is double-submit and exempts GET/HEAD/OPTIONS; auth reads the `gnlm_user_sid` cookie, puts the user in ctx (`pkgcontext.GetUserId`), and returns 401 `ErrNotLogin` when the session is missing.
- **Envelope**: `http.OkResp(c, data)` → 200 `{"code":0,"msg":"ok","data":…}`; `http.ErrResp(c, err)` writes `{code,msg}` at `InnerError.Status`. **Business errors are HTTP 200** — `ErrParams`(1000), `ErrNoRecord`(1001) and every domain error derived from them; only the 2000–2003 builtins produce 401/403; a non-`InnerError` becomes 500 `code:-999`.
- **Request binding**: `c.BindAndValidate(&req)` runs the custom binder in `pkg/http/binder.go` — `path:`/`query:`/`json:` tags, then `validator/v10` tags, then the DTO's own `Validate() error`. A plain validator failure surfaces as 200 `code:1000`.
- **Object URLs are never persisted**: only the encoded `StoreKey` is stored (mapper); every URL is resolved per request. Public-read assets (avatars, style previews) use `adapter.ObjectPublicURLer.PublicURL`; private ones (source content, artifact results) use `PresignGet`. A resolve failure normally degrades to an empty URL plus a WARN, not an API error.
- **Error codes**: 100xxx notebook, 101xxx chat, 102xxx artifact, 103xxx source, 104xxx worker, 105xxx sandbox, 106xxx identity. Each is declared as `errors.ErrXxx.ErrCode(n).Msg(…)`, so its HTTP status is inherited from the builtin it derives from.
- **Wiring**: `ServerDeps` in `server.go` is the only injection surface — add the repo field there, build it in `bootstrap/notelm.go`, construct the application handler in the `NewServer` literal. There is no route manifest to update.

## Cross-process flows

- **Artifact generation**: notelm's `GenerateArtifactHandler` builds `contract.WorkerInput` via `buildWorkerInput` and submits a long task with `flow.Submit` → worker's `entrypoint/worker` decodes it and dispatches on `kind` into `application/worker/artifact/<kind>`; results come back as `WorkerOutput`, and notelm's `artifact/syncer` polls flow task state and persists it.
- **Source ingestion**: notelm publishes the inter-process `source.preparation` event → sourcejob's `PrepareSourceHandler` parses, chunks, and embeds it into Milvus, then emits domain events back.
- **Event publishing**: entities call `AddEvent` inside their mutation methods; handlers call `PullEvents()` after a successful `Save` and hand the events to the eventbus. A publish failure is logged only — it never rolls back saved data.

## Conventions when changing code

- **New persistence capability**: define the interface in `domain/<domain>/repository` → implement in `infrastructure/repository` → convert models in `mapper` → add the GORM model in `infrastructure/database/schema` → wire it up in `bootstrap`.
- **Every DAO file ships with a test file.** A data-access implementation gets a paired `<file>_test.go` in the same package — `postgres/notebookstore.go` → `notebookstore_test.go`, `cache/redis/suggestion.go` → `suggestion_test.go`, `olap/clickhouse/llmlogstore.go` → `llmlogstore_test.go`. The package's `main_test.go` holds a `TestMain` that builds a throwaway database from the `TEST_GONOTELM_*` env vars through `pkg/testsuite/{sql,redis,clickhouse}`, applies the same goose migrations as `cmd/migrate`, and exposes `testXxxStore` values to the tests; `open_test.go` covers the wiring. Assertions use goconvey (`Convey` / `So`).
- **New external component**: register a `Component` constructor in `bootstrap/shared`, then have each `bootstrap/<binary>.go` request only what it needs. Wiring belongs in code, not in config.
- **New config field**: add it to `internal/conf/<binary>.go` and `etc/<binary>.toml.tpl`, with defaults in the `Init()` method; use a goose migration under `migration/db/postgres18` for schema changes.
- **New artifact kind**: add the Kind and payload in `domain/artifact/entity` → add the pipeline in `application/worker/artifact/<kind>` → register it in `interfaces/entrypoint/worker` → list it in `buildinTaskTypes` in `bootstrap/worker.go`.
- Match the surrounding file's comment language (many files carry Chinese comments).

## Verification

Commands live in `Makefile` and `README.md`; the common ones:

- `make gofmt` to format; `go build ./...` to compile everything.
- `make test` for unit tests. Note it **only runs the packages listed in `TEST_PKGS` in the `Makefile`** (postgres / redis / olap / repository / pkg/initjob / migration). Tests under `domain` and `application` reach CI only after being added to `TEST_PKGS`.
- `make migrate` (equivalent to `cmd/migrate -baseline`) migrates the schema; `make dev-migrate` migrates the dev middleware stack.
- Lint matches CI: golangci-lint, version pinned in `.github/workflows/ci.yml`.
