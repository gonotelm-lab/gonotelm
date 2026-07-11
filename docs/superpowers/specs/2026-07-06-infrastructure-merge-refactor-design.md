# Merge infra/ and infrastructure/ into Unified infrastructure/

## Summary

Merge `internal/infra/` and `internal/infrastructure/` into a single `internal/infrastructure/` tree, organized by capability categories (database, cache, mq, storage, vectordb, llm). Each category provides its own interface + config-driven factory function. Eliminate the `Instances` service-locator struct; replace `internal/wire/bootstrap.go` with a single `internal/bootstrap/app.go` function that does explicit constructor injection.

No new features. Pure file migration and import path rewrite.

## Target Structure

```
internal/
├── infrastructure/                  ← unified (was infra/ + infrastructure/)
│   │
│   ├── database/                    ← was infra/dal/
│   │   ├── database.go              ← Store interfaces + DAL struct + Open(cfg)
│   │   ├── schema/                  ← was infra/dal/schema/
│   │   │   ├── notebook.go
│   │   │   ├── source.go
│   │   │   ├── chat.go
│   │   │   └── ...
│   │   └── postgres/                ← was infra/dal/impl/postgres/
│   │       ├── notebook.go
│   │       ├── source.go
│   │       └── ...
│   │
│   ├── cache/                       ← was infra/cache/
│   │   ├── cache.go                 ← Cache interfaces + Cache struct + Open(cfg)
│   │   ├── schema/
│   │   └── redis/                   ← was infra/cache/impl/ (6 files) + infra/cache/redis.go
│   │
│   ├── mq/                          ← was infra/mq/
│   │   ├── mq.go                    ← Producer, Consumer, Message interfaces + MQ struct + Open(cfg)
│   │   └── kafka/                   ← was infra/mq/impl/kafka/
│   │
│   ├── storage/                     ← was infra/storage/
│   │   ├── storage.go               ← Storage interface + Config + Open(cfg)
│   │   └── minio/                   ← was infra/storage/impl/minio/
│   │
│   ├── vectordb/                    ← was infra/vectordal/
│   │   ├── vectordb.go              ← SourceDocStore interface + DAL struct + Open(cfg)
│   │   ├── schema/
│   │   └── milvus/                  ← was infra/vectordal/impl/milvus/
│   │
│   ├── llm/                         ← was infra/llm/ (chat + embedding + rerank + text2image + gateway merged)
│   │   ├── llm.go                   ← all interfaces + Gateway + Open(cfg)
│   │   └── openai/                  ← was infra/llm/gateway/ + infra/llm/chat/impl.go
│   │
│   ├── repository/                  ← was infrastructure/repository/
│   │   ├── notebook.go
│   │   ├── source.go
│   │   ├── chat.go
│   │   └── mapper/                  ← merged from dal/schema/mapper/ + cache/schema/mapper/ + vectordal/schema/mapper/
│   │       ├── notebook.go          ← dal/mapper/notebook.go
│   │       ├── source.go            ← dal/mapper/source.go
│   │       ├── chat.go              ← dal/mapper/chat.go
│   │       ├── message.go           ← dal/mapper/message.go
│   │       ├── message_test.go      ← dal/mapper/message_test.go
│   │       ├── contextmessage.go    ← cache/mapper/contextmessage.go
│   │       ├── streamtask.go        ← cache/mapper/streamtask.go
│   │       └── sourcedoc.go         ← vectordal/mapper/sourcedoc.go
│   │
│   ├── eventbus/                    ← was infrastructure/eventbus/
│   │   ├── bus.go
│   │   ├── inner.go
│   │   ├── outer.go
│   │   └── composite.go
│   │
│   └── adapter/                     ← was infrastructure/adapter/
│       └── summarizer.go
│
├── bootstrap/                       ← was internal/wire/
│   └── app.go                       ← single NewApp(cfg) function
│
├── domain/                          ← unchanged
├── application/                     ← unchanged
├── core/                            ← unchanged
├── api/                             ← unchanged (import paths updated only)
├── app/                             ← unchanged (import paths updated only)
├── conf/                            ← unchanged
└── interfaces/                      ← unchanged (import paths updated only)
```

## Migration Map

### Delete

| Old Path | Reason |
|----------|--------|
| `internal/infra/init.go` | `Instances` struct eliminated, replaced by `bootstrap/app.go` |
| `internal/wire/bootstrap.go` | Replaced by `bootstrap/app.go` |
| `internal/wire/` (entire directory) | Replaced by `bootstrap/` |
| `internal/infrastructure/` (entire directory) | Content moved into `internal/infrastructure/repository/`, `eventbus/`, `adapter/` |

### Move

| From | To |
|------|-----|
| `infra/dal/dal.go` | `infrastructure/database/database.go` |
| `infra/dal/schema/` | `infrastructure/database/schema/` |
| `infra/dal/impl/postgres/` | `infrastructure/database/postgres/` |
| `infra/cache/interface.go` | `infrastructure/cache/cache.go` |
| `infra/cache/redis.go` + `infra/cache/impl/*` | `infrastructure/cache/redis/` (merge 1 + 6 files into one package) |
| `infra/cache/schema/` | `infrastructure/cache/schema/` |
| `infra/mq/mq.go` | `infrastructure/mq/mq.go` |
| `infra/mq/impl/kafka/` | `infrastructure/mq/kafka/` |
| `infra/storage/storage.go` + `config.go` | `infrastructure/storage/storage.go` |
| `infra/storage/impl/minio/` | `infrastructure/storage/minio/` |
| `infra/vectordal/dal.go` | `infrastructure/vectordb/vectordb.go` |
| `infra/vectordal/schema/` | `infrastructure/vectordb/schema/` |
| `infra/vectordal/impl/milvus/` | `infrastructure/vectordb/milvus/` |
| `infra/llm/chat/` + `embedding/` + `rerank/` + `text2image/` + `gateway/` | `infrastructure/llm/` (merge into fewer files) |
| `infra/dal/schema/mapper/{chat,message,notebook,source,message_test}.go` | `infrastructure/repository/mapper/{chat,message,notebook,source,message_test}.go` |
| `infra/cache/schema/mapper/{contextmessage,streamtask}.go` | `infrastructure/repository/mapper/{contextmessage,streamtask}.go` |
| `infra/vectordal/schema/mapper/sourcedoc.go` | `infrastructure/repository/mapper/sourcedoc.go` |
| `infrastructure/adapter/summarizer.go` | `infrastructure/adapter/summarizer.go` |
| `infrastructure/eventbus/` | `infrastructure/eventbus/` |
| `infrastructure/repository/` | `infrastructure/repository/` |
| `wire/bootstrap.go` | `bootstrap/app.go` |

### Removed packages

| Package | Reason |
|---------|--------|
| `internal/infra` root (`init.go`) | `Instances` struct deleted |
| `internal/infra/dal/impl/` | Flat into `database/postgres/` |
| `internal/infra/cache/impl/` | Merge into `cache/redis/` |
| `internal/infra/mq/impl/` | Flat into `mq/kafka/` |
| `internal/infra/storage/impl/` | Flat into `storage/minio/` |
| `internal/infra/vectordal/impl/` | Flat into `vectordb/milvus/` |
| `internal/wire/` | Replaced by `bootstrap/` |

## Config-Driven Factory Pattern

TOML 配置结构完全不变，现有 `conf.Config` 字段保持不变。每个类目包的 `Open()` 直接接受现有的 config struct 类型：

```go
// infrastructure/database/database.go

func Open(cfg conf.DatabaseConfig) (*DAL, error) {
    switch cfg.Type {
    case "postgres":
        return postgres.Open(cfg)
    default:
        return nil, fmt.Errorf("unsupported database driver: %s", cfg.Type)
    }
}
```

Same pattern for `cache/` (接受 `cache.RedisCacheConfig`), `mq/` (接受 `mq.Config`), `storage/` (接受 `storage.Config`), `vectordb/` (接受 `vectordb.Config`), `llm/` (接受 `chat.ProviderConfig`)。

### Config struct (unchanged)

```go
// conf/config.go — 保持平铺，不新增 InfrastructConfig 包装
type Config struct {
    Database  DatabaseConfig         `toml:"database"`
    Redis     cache.RedisCacheConfig `toml:"redis"`
    VectorDB  vectordb.Config        `toml:"vectorDb"`
    Storage   storage.Config         `toml:"storage"`
    MsgQueue  mq.Config              `toml:"msgQueue"`
    Embedding embedding.Config       `toml:"embedding"`
    Rerank    rerank.Config          `toml:"rerank"`
    Provider  chat.ProviderConfig    `toml:"provider"`
    // ...
}
```

TOML 文件和环境变量完全不需要改动（`${GONOTELM_DB_HOST:-...}` 等 envsubst 变量不变）。

## Bootstrap

`internal/bootstrap/app.go` — single function, explicit constructor injection:

```go
package bootstrap

type App struct {
    Server  *api.Server
    closers []io.Closer
}

func (a *App) Close() error {
    for i := len(a.closers) - 1; i >= 0; i-- {
        if err := a.closers[i].Close(); err != nil {
            // log and continue
        }
    }
    return nil
}

func NewApp(cfg *conf.Config) (*App, error) {
    // — 1. infrastructure (config-driven) —
    db, _ := database.Open(cfg.Database)
    vdb, _ := vectordb.Open(cfg.VectorDB)
    cache, _ := cache.Open(cfg.Redis)
    mq, _ := mq.Open(cfg.MsgQueue)
    oss, _ := storage.Open(cfg.Storage)
    llm, _ := llm.Open(cfg.Provider)
    embedding, _ := llm.NewEmbedding(cfg.Embedding)
    rerank, _ := llm.NewReranker(cfg.Rerank)

    // — 2. repositories (implement domain interfaces) —
    notebookRepo := repository.NewNotebookRepo(db, cache)
    sourceRepo := repository.NewSourceRepo(db, vdb, oss)
    chatRepo := repository.NewChatRepo(db, cache)

    // — 3. event bus —
    innerBus := eventbus.NewInner()
    outerBus := eventbus.NewOuter(mq)
    bus := eventbus.NewComposite(innerBus, outerBus)

    // — 4. adapters —
    summarizer := adapter.NewSummarizer(llm)

    // — 5. application handlers —
    createNotebook := appNotebook.NewCreateHandler(notebookRepo, bus)
    getNotebook := appNotebook.NewGetHandler(notebookRepo)
    // ... all handlers

    // — 6. business logic —
    srcLogic := logic.NewSourceLogic(...)
    notebookLogic := logic.NewNotebookLogic(...)

    // — 7. event handler registration —
    eventbus.Register(bus, event.SourcePrepared, appSource.NewPrepHandler(srcLogic))
    // ...

    // — 8. HTTP server —
    srv := api.NewServer(notebookLogic, srcLogic, ...)

    return &App{
        Server:  srv,
        closers: []io.Closer{db, vdb, cache, mq, oss},
    }, nil
}
```

### cmd/main.go simplification

```go
func run() {
    app, err := bootstrap.NewApp(conf.Global())
    if err != nil {
        log.Fatal(err)
    }
    defer app.Close()
    app.Server.Run()
}
```

## Import Path Rewrite Table

| Old Import | New Import |
|------------|------------|
| `internal/infra` | (deleted) |
| `internal/infra/dal` | `internal/infrastructure/database` |
| `internal/infra/dal/schema` | `internal/infrastructure/database/schema` |
| `internal/infra/dal/schema/mapper` | `internal/infrastructure/repository/mapper` |
| `internal/infra/dal/impl/postgres` | `internal/infrastructure/database/postgres` |
| `internal/infra/cache` | `internal/infrastructure/cache` |
| `internal/infra/cache/schema` | `internal/infrastructure/cache/schema` |
| `internal/infra/cache/schema/mapper` | `internal/infrastructure/repository/mapper` |
| `internal/infra/mq` | `internal/infrastructure/mq` |
| `internal/infra/mq/impl/kafka` | `internal/infrastructure/mq/kafka` |
| `internal/infra/storage` | `internal/infrastructure/storage` |
| `internal/infra/storage/impl/minio` | `internal/infrastructure/storage/minio` |
| `internal/infra/vectordal` | `internal/infrastructure/vectordb` |
| `internal/infra/vectordal/schema` | `internal/infrastructure/vectordb/schema` |
| `internal/infra/vectordal/schema/mapper` | `internal/infrastructure/repository/mapper` |
| `internal/infra/vectordal/impl/milvus` | `internal/infrastructure/vectordb/milvus` |
| `internal/infra/llm/chat` | `internal/infrastructure/llm` |
| `internal/infra/llm/embedding` | `internal/infrastructure/llm` |
| `internal/infra/llm/rerank` | `internal/infrastructure/llm` |
| `internal/infra/llm/text2image` | `internal/infrastructure/llm` |
| `internal/infra/llm/gateway` | `internal/infrastructure/llm/openai` |
| `internal/infrastructure/adapter` | `internal/infrastructure/adapter` |
| `internal/infrastructure/eventbus` | `internal/infrastructure/eventbus` |
| `internal/infrastructure/repository` | `internal/infrastructure/repository` |
| `internal/wire` | `internal/bootstrap` |

## Type Renames

| Old Name | New Name | Reason |
|----------|----------|--------|
| `dal.DAL` | `database.DAL` | More explicit |
| `vectordal.DAL` | `vectordb.DAL` | More explicit |
| `infra.Instances` | (deleted) | Eliminate service locator |
| `*wire.Wire` | `*bootstrap.App` | Bootstrap owns the assembled app |

## Validation Criteria

1. `go build ./...` passes with zero errors
2. `go vet ./...` passes
3. All existing tests pass unchanged
4. Only new file: `bootstrap/app.go`. All other files are moved/renamed, with `Open(cfg)` factory added to each category root file (`database/database.go`, `cache/cache.go`, etc.)
5. No functional behavior changed
6. `git grep "internal/infra"` returns zero results
7. `git grep "internal/infrastructure"` returns only expected paths under `internal/infrastructure/`
8. `git grep "internal/wire"` returns zero results
