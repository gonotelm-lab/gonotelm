# Infrastructure Merge Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Merge `internal/infra/` and `internal/infrastructure/` into a single `internal/infrastructure/` tree organized by capability categories, eliminate the `Instances` service-locator struct, replace `wire/bootstrap.go` with `bootstrap/app.go` single-function constructor injection.

**Architecture:** Each capability category (database, cache, mq, storage, vectordb, llm) gets a root file with interfaces + `Open(cfg)` factory and a subdirectory per technology implementation. Mappers merge from three scattered locations into `repository/mapper/`. `bsrapot/bootap.App` single function builds the entire object graph via explicit constructor injection.

**Tech Stack:** Go 1.25.4, no new dependencies.

## Global Constraints

- **Dead code removal**: Before migrating any file, verify it has active consumers via `rg`. Dead code is removed rather than migrated.
- Build and tests pass after final Task 14. Intermediate tasks (1-13) involve package moves that temporarily break compilation — this is expected and accepted.
- After Task 14: `go build ./...`, `go vet ./...`, `go test ./...` all pass.
- Only new file: `internal/bootstrap/app.go`. All other files are moved/renamed with `Open(cfg)` factory functions added to category root files
- `conf.Config` struct and TOML structure remain flat and unchanged
- Environment variables (${GONOTELM_*} in etc/gonotelm.toml.tpl) remain unchanged
- No import of `internal/infra` or `internal/wire` remaining after migration
- No functional behavior changed

---

## Task Structure

Each task breaks down into multiple steps. Tasks 1-6 move files by capability category. Task 7 merges mappers. Task 8-10 migrate the former `infrastructure/` content. Task 11 creates bootstrap. Tasks 12-14 do global import rewrite and final cleanup.

### Task 1: Create `infrastructure/database/` category

**Files:**
- Create: `internal/infrastructure/database/database.go` (moved from `internal/infra/dal/dal.go`)
- Create: `internal/infrastructure/database/schema/` (moved from `internal/infra/dal/schema/`, dead code removed)
- Create: `internal/infrastructure/database/postgres/` (moved from `internal/infra/dal/impl/postgres/`)
- Delete after Task 14: `internal/infra/dal/`

**Interfaces:**
- Consumes: `conf.DatabaseConfig` (unchanged, type field is `Type string`)
- Produces: `database.DAL` struct, `database.NotebookStore` (was `dal.NotebookStore`), `database.SourceStore`, `database.ChatStore`, `database.ChatMessageStore`, `database.ArtifactTaskStore`, `database.Open(cfg conf.DatabaseConfig) (*DAL, error)`

- [ ] **Step 1: Dead code detection**

Check which schema types and Store interfaces in `infra/dal/` are actually imported by files outside `internal/infra/`. Only migrate files that have active consumers.

```bash
# Check schema file usage from outside infra/
for f in internal/infra/dal/schema/*.go; do
    name=$(basename "$f" .go)
    refs=$(rg -l "$name" --go internal/ | rg -v "internal/infra/dal" | wc -l)
    echo "$name: $refs external refs"
done

# Check Store interface usage outside infra/
rg -l "dal\.(NotebookStore|SourceStore|ChatStore|ChatMessageStore|ArtifactTaskStore)" --go internal/ \
  | rg -v "internal/infra/dal"
```

Remove any schema file or Store interface with zero external references — do NOT migrate dead code.

- [ ] **Step 2: Create directory structure**

```bash
mkdir -p internal/infrastructure/database/schema
mkdir -p internal/infrastructure/database/postgres
```

- [ ] **Step 2: Move dal.go, rename package, add Open()**

```bash
cp internal/infra/dal/dal.go internal/infrastructure/database/database.go
```

Edit `internal/infrastructure/database/database.go`:
- Change `package dal` → `package database`
- Remove the import of `dalimpl` (if present, likely not since dal.go is the interface file)
- Append the `Open()` function at the end of the file:

```go
// Import to add:
import "github.com/gonotelm-lab/gonotelm/internal/conf"

func Open(cfg conf.DatabaseConfig) (*DAL, error) {
	switch cfg.Type {
	case "postgres":
		return postgres.Open(cfg)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Type)
	}
}
```

- [ ] **Step 3: Move postgres implementation files**

```bash
cp internal/infra/dal/impl/postgres/*.go internal/infrastructure/database/postgres/
```

In `internal/infrastructure/database/postgres/`, update internal imports in each `.go` file:
- `"github.com/gonotelm-lab/gonotelm/internal/infra/dal"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"`
- `"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"`

Also add `Open()` factory function in a new or existing file (e.g., `postgres.go`):

```go
import "github.com/gonotelm-lab/gonotelm/internal/conf"

func Open(cfg conf.DatabaseConfig) (*database.DAL, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return database.NewDAL(db), nil
}
```

Note: `database.NewDAL()` is already defined in `database/database.go` (was `dal.NewDAL()`). If there was a constructor function like `New(t dal.Type, cfg conf.SQLConfig) (*DAL, error)` in the old `dalimpl` package, move that logic into `Open()` above.

- [ ] **Step 4: Move schema files**

```bash
cp internal/infra/dal/schema/*.go internal/infrastructure/database/schema/
```

Package stays `package schema`. No changes needed to these files (they contain pure data types).

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/database/
git commit -m "refactor: move dal/ to infrastructure/database/ category"
```

---

### Task 2: Create `infrastructure/cache/` category

**Files:**
- Create: `internal/infrastructure/cache/cache.go` (merged from `infra/cache/interface.go` + `infra/cache/redis.go`)
- Create: `internal/infrastructure/cache/schema/` (moved from `infra/cache/schema/`, dead code removed)
- Create: `internal/infrastructure/cache/redis/` (merged from `infra/cache/impl/` with package rename `impl` → `redis`)
- Delete after Task 14: `internal/infra/cache/`

**Interfaces:**
- Consumes: `cache.RedisCacheConfig` (defined in the same file, unchanged)
- Produces: `cache.Cache` struct holding `ChatMessageContextCache` + `ChatMessageStreamCache` interfaces, `cache.Open(cfg cache.RedisCacheConfig) (*Cache, error)`

- [ ] **Step 1: Dead code detection**

```bash
# Check cache schema file usage from outside infra/
for f in internal/infra/cache/schema/*.go; do
    name=$(basename "$f" .go)
    refs=$(rg -l "$name" --go internal/ | rg -v "internal/infra/cache" | wc -l)
    echo "$name: $refs external refs"
done

# Check cache impl types used outside infra/
rg -l "ChatMessageContextCache|ChatMessageStreamCache" --go internal/ | rg -v "internal/infra/cache/impl"
```

Remove any file with zero external references.

- [ ] **Step 2: Create directory structure**

```bash
mkdir -p internal/infrastructure/cache/schema
mkdir -p internal/infrastructure/cache/redis
```

- [ ] **Step 2: Merge interface.go + redis.go into cache.go**

```bash
# Combine the two files
cat internal/infra/cache/interface.go > internal/infrastructure/cache/cache.go
echo "" >> internal/infrastructure/cache/cache.go
cat internal/infra/cache/redis.go >> internal/infrastructure/cache/cache.go
```

Edit `internal/infrastructure/cache/cache.go`:
- Package stays `package cache`
- Ensure there's no duplicate `package cache` line
- Update imports: the `redis.go` part imports `cacheimpl`, which was `infra/cache/impl`. After migration, these types live in `cache/redis/`, so update:
  - `"github.com/gonotelm-lab/gonotelm/internal/infra/cache/impl"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/redis"`
  - `cacheimpl.ChatMessageContextCache` → `redis.ChatMessageContextCache` (or just use local type names if they exist)

- [ ] **Step 3: Add Open() to cache.go**

At the bottom of `infrastructure/cache/cache.go`, add:

```go
func Open(cfg RedisCacheConfig) (*Cache, error) {
	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    cfg.Addrs,
		Username: cfg.Username,
		Password: cfg.Password,
	})
	// Verify connection
	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return &Cache{
		ChatMessageContextCache:  redis.NewChatMessageContextCache(rdb),
		ChatMessageStreamCache:   redis.NewChatMessageStreamCache(rdb),
	}, nil
}
```

Note: the exact factory calls depend on what `redis/` package exports. Adjust after Step 4.

- [ ] **Step 4: Move impl files → redis/, rename package**

```bash
cp internal/infra/cache/impl/*.go internal/infrastructure/cache/redis/
```

In every `.go` file under `internal/infrastructure/cache/redis/`:
- Change `package impl` → `package redis`
- Update imports:
  - `"github.com/gonotelm-lab/gonotelm/internal/infra/cache"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"`
  - `"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"`

- [ ] **Step 5: Move schema files**

```bash
cp internal/infra/cache/schema/*.go internal/infrastructure/cache/schema/
```

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/cache/
git commit -m "refactor: move cache/ to infrastructure/cache/ category"
```

---

### Task 3: Create `infrastructure/mq/` and `infrastructure/storage/` categories

**Files:**
- Create: `internal/infrastructure/mq/mq.go` (from `infra/mq/mq.go`, add Open())
- Create: `internal/infrastructure/mq/kafka/` (from `infra/mq/impl/kafka/`, rename package)
- Create: `internal/infrastructure/storage/storage.go` (from `infra/storage/storage.go` + `config.go`, add Open())
- Create: `internal/infrastructure/storage/minio/` (from `infra/storage/impl/minio/`, rename package)
- Delete after Task 14: `internal/infra/mq/`, `internal/infra/storage/`

- [ ] **Step 1: Dead code detection**

```bash
# Check mq types used outside infra/
rg -l "mq\.(Producer|Consumer|Message|MQ)" --go internal/ | rg -v "internal/infra/mq"

# Check storage types used outside infra/
rg -l "storage\.(Storage|Config)" --go internal/ | rg -v "internal/infra/storage"

# Check if any kafka files reference unused config fields
rg -l "infra/mq/impl/kafka" --go internal/ | rg -v "internal/infra/mq"
```

Remove any unused types, interfaces, or configuration fields.

- [ ] **Step 2: Create directories**

```bash
mkdir -p internal/infrastructure/mq/kafka
mkdir -p internal/infrastructure/storage/minio
```

- [ ] **Step 2: Move mq.go, add Open()**

```bash
cp internal/infra/mq/mq.go internal/infrastructure/mq/mq.go
```

Edit `internal/infrastructure/mq/mq.go`:
- Package stays `package mq`
- Update import: `"github.com/gonotelm-lab/gonotelm/internal/infra/mq/impl"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq/kafka"`
- Append `Open()`:

```go
import "github.com/gonotelm-lab/gonotelm/internal/conf"

func Open(cfg conf.MQConfig) (*MQ, error) {
	switch cfg.Type {
	case "kafka":
		return kafka.Open(cfg.Kafka)
	default:
		return nil, fmt.Errorf("unsupported mq driver: %s", cfg.Type)
	}
}
```

Note: adjust `conf.MQConfig` to the actual type used (currently `mqimpl.Config` with `toml:"msgQueue"`).

- [ ] **Step 3: Move kafka files**

```bash
cp internal/infra/mq/impl/kafka/*.go internal/infrastructure/mq/kafka/
```

Edit each file: change `package kafka` stays (it's already `package kafka`). Update imports:
- `"github.com/gonotelm-lab/gonotelm/internal/infra/mq"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"`

- [ ] **Step 4: Move storage files, add Open()**

```bash
cp internal/infra/storage/storage.go internal/infrastructure/storage/storage.go
# append config.go content
cat internal/infra/storage/config.go >> internal/infrastructure/storage/storage.go
```

Edit `internal/infrastructure/storage/storage.go`:
- Package stays `package storage`
- Remove duplicate `package` line if config.go had one
- Update imports:
  - `"github.com/gonotelm-lab/gonotelm/internal/infra/storage/impl"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage/minio"`
- Append `Open()`:

```go
func Open(cfg Config) (Storage, error) {
	switch cfg.Type {
	case "minio":
		return minio.Open(cfg.Minio)
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", cfg.Type)
	}
}
```

- [ ] **Step 5: Move minio files**

```bash
cp internal/infra/storage/impl/minio/*.go internal/infrastructure/storage/minio/
```

Edit each file: `package minio` stays. Update imports:
- `"github.com/gonotelm-lab/gonotelm/internal/infra/storage"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"`

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/mq/ internal/infrastructure/storage/
git commit -m "refactor: move mq/ and storage/ to infrastructure/ categories"
```

---

### Task 4: Create `infrastructure/vectordb/` category

**Files:**
- Create: `internal/infrastructure/vectordb/vectordb.go` (from `infra/vectordal/dal.go`, rename package)
- Create: `internal/infrastructure/vectordb/schema/` (from `infra/vectordal/schema/`, dead code removed)
- Create: `internal/infrastructure/vectordb/milvus/` (from `infra/vectordal/impl/milvus/`)
- Delete after Task 14: `internal/infra/vectordal/`

- [ ] **Step 1: Dead code detection**

```bash
# Check vectordal schema usage
for f in internal/infra/vectordal/schema/*.go; do
    name=$(basename "$f" .go)
    refs=$(rg -l "$name" --go internal/ | rg -v "internal/infra/vectordal" | wc -l)
    echo "$name: $refs external refs"
done

# Check SourceDocStore usage
rg -l "SourceDocStore" --go internal/ | rg -v "internal/infra/vectordal"
```

Remove any schema type with zero external references.

- [ ] **Step 2: Create directories**

```bash
mkdir -p internal/infrastructure/vectordb/schema
mkdir -p internal/infrastructure/vectordb/milvus
```

- [ ] **Step 2: Move dal.go → vectordb.go, rename package, add Open()**

```bash
cp internal/infra/vectordal/dal.go internal/infrastructure/vectordb/vectordb.go
```

Edit `internal/infrastructure/vectordb/vectordb.go`:
- Change `package vectordal` → `package vectordb`
- Update imports:
  - `"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/impl"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/milvus"`
  - `"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/schema"`
- Append `Open()`:

```go
func Open(cfg conf.VectorDBConfig) (*DAL, error) {
	switch cfg.Type {
	case "milvus":
		return milvus.Open(cfg)
	default:
		return nil, fmt.Errorf("unsupported vectordb driver: %s", cfg.Type)
	}
}
```

Note: `conf.VectorDBConfig` needs adjustment — currently `VectorDB vecimpl.Config` in Config struct. The import path for vecimpl.Config will change to vectordb.Config.

- [ ] **Step 3: Move milvus files**

```bash
cp internal/infra/vectordal/impl/milvus/*.go internal/infrastructure/vectordb/milvus/
```

Edit each file: `package milvus` stays. Update imports:
- `"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"`
- `"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/schema"`

- [ ] **Step 4: Move schema files**

```bash
cp internal/infra/vectordal/schema/*.go internal/infrastructure/vectordb/schema/
```

Package stays `package schema`.

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/vectordb/
git commit -m "refactor: move vectordal/ to infrastructure/vectordb/ category"
```

---

### Task 5: Create `infrastructure/llm/` category

**Files:**
- Create: `internal/infrastructure/llm/llm.go` (consolidated interfaces from chat/, embedding/, rerank/, text2image/)
- Create: `internal/infrastructure/llm/openai/` (consolidated from gateway/ + chat/impl.go)
- Delete after Task 14: `internal/infra/llm/`

**Note:** The LLM packages have provider configs for many providers (embedding: Ark, DashScope, Gemini, Ollama, OpenAI, Qianfan, TencentCloud; chat: DeepSeek, OpenAI, Qwen, Agnes; rerank: DashScope; text2image: DashScope, Agnes). **Only migrate provider configs/types that are actually instantiated or referenced outside the infra/ package.**

- [ ] **Step 1: Dead code detection — find which providers are actually used**

```bash
# Check each embedding provider's usage outside infra/llm/
rg -l "embedding\.(Ark|DashScope|Gemini|Ollama|OpenAI|Qianfan|TencentCloud)" --go internal/ | rg -v "internal/infra/llm/embedding"

# Check each chat provider's usage outside infra/llm/
rg -l "chat\.(Openai|DeepSeek|Qwen|Agnes)" --go internal/ | rg -v "internal/infra/llm/chat"
rg -l "Provider.*openai\|Provider.*deepseek\|Provider.*qwen\|Provider.*agnes" --go -i internal/

# Check rerank usage
rg -l "rerank\." --go internal/ | rg -v "internal/infra/llm/rerank"

# Check text2image usage
rg -l "text2image\." --go internal/ | rg -v "internal/infra/llm/text2image"

# Check gateway usage
rg -l "gateway\.(Gateway|New)" --go internal/ | rg -v "internal/infra/llm/gateway"
```

**Only migrate files containing types that have active external consumers.** Remove dead provider configs completely — don't migrate them.

- [ ] **Step 2: Create directory and examine current structure**

```bash
mkdir -p internal/infrastructure/llm/openai
```

Read each file under `internal/infra/llm/chat/`, `embedding/`, `gateway/`, `rerank/`, `text2image/` to understand the exact types and interfaces.

- [ ] **Step 3: Consolidate chat/ interfaces**

```bash
cp internal/infra/llm/chat/config.go internal/infrastructure/llm/
cp internal/infra/llm/chat/runtime_options.go internal/infrastructure/llm/
cp internal/infra/llm/chat/impl.go internal/infrastructure/llm/openai/
```

Note: `chat/constants.go` and `chat/streamhandle.go` are NOT copied here — they will be extracted to `pkg/` in Task 5b.

In all moved files under `infrastructure/llm/`:
- Change `package chat` → `package llm`
- Update internal imports referencing other moved files

In `infrastructure/llm/openai/`:
- Files stay `package openai` or rename as needed
- Update imports referencing old `infra/llm/chat` → `infrastructure/llm`

- [ ] **Step 3: Consolidate embedding/, rerank/, text2image/**

```bash
# Similar approach — move each sub-package's files into infrastructure/llm/
# and change their package declarations to `package llm`
cp internal/infra/llm/embedding/*.go internal/infrastructure/llm/
cp internal/infra/llm/rerank/*.go internal/infrastructure/llm/
cp internal/infra/llm/text2image/*.go internal/infrastructure/llm/
```

In each moved file:
- Change package declaration to `package llm`
- Keep exported type names (different packages already had distinct names)

- [ ] **Step 4: Move gateway/ files**

```bash
cp internal/infra/llm/gateway/*.go internal/infrastructure/llm/openai/
```

In moved files: change `package gateway` → `package openai`. Update imports.

- [ ] **Step 5: Add Open() to llm.go**

```go
// infrastructure/llm/llm.go

func OpenGateway(cfg ProviderConfig) (*Gateway, error) {
	return openai.NewGateway(&cfg)
}

func OpenEmbedding(cfg EmbeddingConfig) (*EmbeddingGateway, error) {
	return openai.NewEmbeddingGateway(&cfg)
}

func OpenReranker(cfg RerankConfig) (*Reranker, error) {
	return openai.NewReranker(&cfg)
}

func OpenText2Image(cfg Text2ImageConfig) (*Text2ImageGateway, error) {
	return openai.NewText2ImageGateway(&cfg)
}
```

Note: Exact function names and types depend on what exists in the actual code. The principle is that `Open*()` factory functions in `llm/` delegate to `openai/` implementations based on config.

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/llm/
git commit -m "refactor: move llm/ to infrastructure/llm/ category"
```

---

### Task 5b: Extract shared LLM utilities to `pkg/`

Three chunks of code in the llm packages are completely provider-agnostic and duplicated across sub-packages. Extract them to `pkg/` before finalizing the migration.

**Candidate 1: Stream handler → `pkg/eino-ext/stream/`**

`internal/infra/llm/chat/streamhandle.go` (390 lines) handles streaming responses from any eino chat model. Zero provider-specific logic. It defines `HandleStream`, `HandleStreamWithCallback`, `EventType`, `StreamError`, `Callbacks`, `PackedContent`, `streamTracker`.

```bash
mkdir -p pkg/eino-ext/stream
cp internal/infra/llm/chat/streamhandle.go pkg/eino-ext/stream/stream.go
cp internal/infra/llm/chat/streamhandle_test.go pkg/eino-ext/stream/stream_test.go
```

Edit `pkg/eino-ext/stream/stream.go`:
- Change `package chat` → `package stream`
- Update import of `finishReasonStop`, etc. — either inline the constants or import from `pkg/llm/`
- Remove any chat-specific imports (`internal/infra/llm/chat` should not be imported)

**Candidate 2: Finish reason constants → `pkg/llm/constants.go`**

`internal/infra/llm/chat/constants.go` defines OpenAI-compatible finish reasons used by both `streamhandle.go` and `interceptor.go`.

```bash
mkdir -p pkg/llm
cp internal/infra/llm/chat/constants.go pkg/llm/constants.go
```

Edit: change `package chat` → `package llm`.

**Candidate 3: OpenAI-compatible extra fields → `pkg/eino-ext/openai/`**

`internal/infra/llm/gateway/extrafield.go` (`streamOptionsIncludeUsage`) and `internal/infra/llm/chat/runtime_options.go` (`responseFormatJsonObject`) are OpenAI-compatible API conventions used by multiple providers (OpenAI, DeepSeek, Qwen).

```bash
mkdir -p pkg/eino-ext/openai
```

Create `pkg/eino-ext/openai/extrafield.go`:
```go
package openai

var StreamOptionsIncludeUsage = map[string]any{
    "stream_options": map[string]bool{
        "include_usage": true,
    },
}

var ResponseFormatJSONObject = map[string]any{
    "response_format": map[string]string{
        "type": "json_object",
    },
}
```

- [ ] **Step 1: Extract stream handler**

```bash
mkdir -p pkg/eino-ext/stream
cp internal/infra/llm/chat/streamhandle.go pkg/eino-ext/stream/stream.go
cp internal/infra/llm/chat/streamhandle_test.go pkg/eino-ext/stream/stream_test.go
```

Edit `pkg/eino-ext/stream/stream.go`:
- Change `package chat` → `package stream`
- Replace inline finish reason references with constants from `pkg/llm/` (after Step 2)

Edit `pkg/eino-ext/stream/stream_test.go`:
- Change `package chat` → `package stream`

- [ ] **Step 2: Extract constants**

```bash
mkdir -p pkg/llm
```

Create `pkg/llm/constants.go`:
```go
package llm

const (
    FinishReasonStop          = "stop"
    FinishReasonLength        = "length"
    FinishReasonToolCalls     = "tool_calls"
    FinishReasonContentFilter = "content_filter"
)
```

- [ ] **Step 3: Extract extra fields**

```bash
mkdir -p pkg/eino-ext/openai
```

Create `pkg/eino-ext/openai/extrafield.go` with `StreamOptionsIncludeUsage` and `ResponseFormatJSONObject`.

- [ ] **Step 4: Update references in `infrastructure/llm/`**

After extraction, update imports in the moved `infrastructure/llm/` files:
- `chat/streamhandle.go` references → `pkg/eino-ext/stream`
- `chat/constants.go` references → `pkg/llm`
- `gateway/extrafield.go` + `chat/runtime_options.go` references → `pkg/eino-ext/openai`

- [ ] **Step 5: Update `infrastructure/llm/openai/` to use new pkg paths**

Files under `infrastructure/llm/openai/` (was gateway/) that call `HandleStreamWithCallback`, references `Callbacks`, `PackedContent`, etc. should import from `pkg/eino-ext/stream` instead.

- [ ] **Step 6: Commit**

```bash
git add pkg/eino-ext/stream/ pkg/eino-ext/openai/ pkg/llm/
git add internal/infrastructure/llm/
git commit -m "refactor: extract shared LLM utils to pkg/ (stream, constants, extra fields)"
```

---

### Task 6: Migrate mapper files to `repository/mapper/`

**Files:**
- Create: `internal/infrastructure/repository/mapper/` — only mapper files that have active consumers outside `infra/`

All three source packages use `package mapper` — no rename needed. But imports within each file must be updated.

- [ ] **Step 1: Dead code detection**

```bash
# Check which mapper functions are used outside infra/ and old infrastructure/
rg -l "mapper\." --go internal/ | rg -v "internal/infra/.*/mapper" | rg -v "internal/infrastructure/repository"
```

Any mapper function with zero external callers is dead code — do NOT migrate.

- [ ] **Step 2: Ensure directory exists**

```bash
mkdir -p internal/infrastructure/repository/mapper
```

- [ ] **Step 2: Copy all mapper files**

```bash
cp internal/infra/dal/schema/mapper/*.go internal/infrastructure/repository/mapper/
cp internal/infra/cache/schema/mapper/*.go internal/infrastructure/repository/mapper/
cp internal/infra/vectordal/schema/mapper/*.go internal/infrastructure/repository/mapper/
```

- [ ] **Step 3: Update imports in each moved mapper file**

In files from `dal/schema/mapper/`:
- `"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"`

In files from `cache/schema/mapper/`:
- `"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"`

In `sourcedoc.go` from `vectordal/schema/mapper/`:
- `"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/schema"`

All domain imports (e.g., `internal/domain/source/entity`) stay unchanged — only infra-layer imports change.

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/repository/mapper/
git commit -m "refactor: consolidate mappers into repository/mapper/"
```

---

### Task 7: Migrate `infrastructure/repository/`, `eventbus/`, `adapter/`

**Files:**
- Move: `internal/infrastructure/repository/*.go` → into new `internal/infrastructure/repository/` (already in the right place, only update imports)
- Move: `internal/infrastructure/eventbus/*.go` → into `internal/infrastructure/eventbus/` (same location, update imports)
- Move: `internal/infrastructure/adapter/summarizer.go` → into `internal/infrastructure/adapter/` (same location, update imports)

These files currently import from `internal/infra/dal`, `internal/infra/vectordal`, etc. After Tasks 1-5, their import targets have moved. Update imports now.

- [ ] **Step 1: Dead code detection**

```bash
# Check which repository functions have external consumers
rg -l "repository\.New" --go internal/ | rg -v "internal/infrastructure/repository"

# Check eventbus usage
rg -l "eventbus\." --go internal/ | rg -v "internal/infrastructure/eventbus"

# Check adapter usage
rg -l "adapter\." --go internal/ | rg -v "internal/infrastructure/adapter"
```

Remove any file whose exported types have zero external consumers.

- [ ] **Step 2: Update imports in `infrastructure/repository/` files**

In each `.go` file under `internal/infrastructure/repository/`:
- `"github.com/gonotelm-lab/gonotelm/internal/infra/dal"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"`
- `"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"`
- `"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema/mapper"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"`
- `"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"`
- `"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/schema"`
- `"github.com/gonotelm-lab/gonotelm/internal/infra/cache"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"`
- `"github.com/gonotelm-lab/gonotelm/internal/infra/storage"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"`

Also update type references:
- `dal.NotebookStore` → `database.NotebookStore`
- `dal.SourceStore` → `database.SourceStore`
- `dal.ChatStore` → `database.ChatStore`
- `dal.ChatMessageStore` → `database.ChatMessageStore`
- `dal.ArtifactTaskStore` → `database.ArtifactTaskStore`
- `vectordal.SourceDocStore` → `vectordb.SourceDocStore`

- [ ] **Step 3: Update imports in `infrastructure/eventbus/` files**

In each `.go` file under `internal/infrastructure/eventbus/`:
- `"github.com/gonotelm-lab/gonotelm/internal/infra/mq"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"`
- Type: `mq.Producer` → `mq.Producer` (same, no change), `mq.Consumer` → `mq.Consumer`
- `"github.com/gonotelm-lab/gonotelm/internal/infra/storage"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"` (if used)

- [ ] **Step 4: Update imports in `infrastructure/adapter/summarizer.go`**

- `"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"`
- `"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"` → `"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/openai"`
- Type: `chat.Provider` → `llm.Provider`

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/repository/ internal/infrastructure/eventbus/ internal/infrastructure/adapter/
git commit -m "refactor: update imports in repository/, eventbus/, adapter/"
```

---

### Task 8: Create `bootstrap/app.go`

**Files:**
- Create: `internal/bootstrap/app.go`
- Reference: `cmd/main.go` (for the current wiring flow), `internal/wire/bootstrap.go` (for the existing Wire struct construction logic)

**The `App` struct replaces both `*infra.Instances` and `*wire.Wire`.** It owns the closers, the server, and the lifecycle.

- [ ] **Step 1: Create directory and file**

```bash
mkdir -p internal/bootstrap
```

Write `internal/bootstrap/app.go`:

```go
package bootstrap

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/api"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic"
	sourcelogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/source"
	studiologic "github.com/gonotelm-lab/gonotelm/internal/app/logic/studio"
	"github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizartifact "github.com/gonotelm-lab/gonotelm/internal/app/biz/artifact"
	bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"
)

type App struct {
	Server  *api.Server
	closers []io.Closer
}

func (a *App) Close() error {
	for i := len(a.closers) - 1; i >= 0; i-- {
		if err := a.closers[i].Close(); err != nil {
			slog.Error("close error", "err", err)
		}
	}
	return nil
}

func NewApp(ctx context.Context, cfg *conf.Config) (*App, error) {
	// ── 1. Infrastructure ──

	db, err := database.Open(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	vdb, err := vectordb.Open(cfg.VectorDB)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("vectordb: %w", err)
	}

	cacheInst, err := cache.Open(cfg.Redis)
	if err != nil {
		vdb.Close()
		db.Close()
		return nil, fmt.Errorf("cache: %w", err)
	}

	mqInst, err := mq.Open(cfg.MsgQueue)
	if err != nil {
		cacheInst.Close()
		vdb.Close()
		db.Close()
		return nil, fmt.Errorf("mq: %w", err)
	}

	oss, err := storage.Open(cfg.Storage)
	if err != nil {
		mqInst.Close()
		cacheInst.Close()
		vdb.Close()
		db.Close()
		return nil, fmt.Errorf("storage: %w", err)
	}

	llmGateway, err := llm.OpenGateway(&cfg.Provider)
	if err != nil {
		oss.Close()
		mqInst.Close()
		cacheInst.Close()
		vdb.Close()
		db.Close()
		return nil, fmt.Errorf("llm gateway: %w", err)
	}

	embeddingGateway, err := llm.OpenEmbedding(&cfg.Embedding)
	if err != nil {
		llmGateway.Close()
		oss.Close()
		mqInst.Close()
		cacheInst.Close()
		vdb.Close()
		db.Close()
		return nil, fmt.Errorf("embedding: %w", err)
	}

	// ── 2. Repositories ──

	notebookRepo := repository.NewNotebookRepository(db.NotebookStore, db.SourceStore)
	sourceRepo := repository.NewSourceRepository(db.SourceStore)
	sourceStorageRepo := repository.NewSourceStorageRepository(oss)
	sourceDocRepo := repository.NewSourceDocRepository(embeddingGateway.Embedder(), vdb.SourceDocStore, repository.SourceDocRepositoryConfig{})
	chatRepo := repository.NewChatRepository(db.ChatStore)
	messageRepo := repository.NewMessageRepository(db.ChatMessageStore)
	contextMsgRepo := repository.NewContextMessageRepository(cacheInst.ChatMessageContextCache)
	streamTaskRepo := repository.NewStreamTaskRepository(cacheInst.ChatMessageStreamCache)
	artifactTaskRepo := repository.NewArtifactTaskRepository(db.ArtifactTaskStore)

	// ── 3. Event Bus ──

	innerBus := eventbus.NewInnerEventBus()
	outerBus := eventbus.NewOuterEventBus(mqInst)
	bus := eventbus.NewCompositeEventBus(innerBus, outerBus)

	// ── 4. Adapters ──

	summarizer := adapter.NewSummarizer(llmGateway)

	// ── 5. Biz objects ──

	prompt := bizprompt.New("zh")
	notebookBiz := biznotebook.New(db.NotebookStore)
	artifactBiz := bizartifact.New(db.ArtifactTaskStore)
	sourceBiz, err := bizsource.New(oss, db.SourceStore, vdb.SourceDocStore, llmGateway, embeddingGateway, prompt)
	if err != nil {
		return nil, fmt.Errorf("source biz: %w", err)
	}
	agentSourceBiz, err := bizsource.NewAgentBiz(ctx, sourceBiz, bizsource.AgentBizConfig{})
	if err != nil {
		return nil, fmt.Errorf("agent source biz: %w", err)
	}

	// ── 6. Logic ──

	srcLogic := sourcelogic.MustNewLogic(ctx, mqInst, cacheInst.Redis, oss, notebookBiz, sourceBiz, llmGateway, prompt)
	studioLogic := studiologic.MustNewLogic(ctx, oss, sourceBiz, agentSourceBiz, notebookBiz, artifactBiz, llmGateway, text2imageGateway, prompt)

	// ── 7. Event handler registration ──

	// event.Init(ctx, bus, ...) — copy from wire/bootstrap.go and interfaces/event/, adapt params

	// ── 8. HTTP Server ──

	srv := api.NewServer(logicInst, api.ServerDeps{
		NotebookRepo:      notebookRepo,
		SourceRepo:        sourceRepo,
		SourceStorageRepo: sourceStorageRepo,
		SourceDocRepo:     sourceDocRepo,
		ChatRepo:          chatRepo,
		MessageRepo:       messageRepo,
		ContextMessageRepo: contextMsgRepo,
		StreamTaskRepo:    streamTaskRepo,
		EventBus:          bus,
		Summarizer:        summarizer,
	})

	return &App{
		Server:  srv,
		closers: []io.Closer{oss, mqInst, cacheInst, vdb, db},
	}, nil
}
```

**IMPORTANT: Before writing this file, read these files to verify every constructor signature and field name:**

1. `internal/wire/bootstrap.go` — the **existing** wiring code. Mirror its exact logic, just with new package paths.
2. `internal/infrastructure/repository/sourcedoc.go` — `NewSourceDocRepository` takes `(embedding.Embedder, vdal.SourceDocStore, SourceDocRepositoryConfig)`. The `embedder` comes from `embeddingGateway.Embedder()` (verify the method name — it may be `Generator()` or similar).
3. `internal/infra/cache/impl/cache.go` — verify `Cache` struct field names: `ChatMessageContextCache` / `ChatMessageStreamCache` / `Redis`. The exact field names determine how `cacheInst.*` is accessed in the bootstrap.
4. `internal/infra/llm/embedding/gateway.go` — verify the `Gateway` struct and embedder accessor method.
5. `internal/infra/llm/text2image/gateway.go` — verify the `Gateway` struct; must also create `text2imageGateway` from `cfg.Text2Image` and pass it to `studiologic.MustNewLogic`.
6. `internal/api/server.go` — adjust `ServerDeps` to match the exact fields currently accessed from `wire.Wire`.
7. `internal/interfaces/event/eventhandler.go` — the `Init` function; adapt from accepting `*wire.Wire` to accepting specific types (EventBus, handlers).

**Additional factory that must be created in the infrastructure step:**

```go
text2imageGateway, err := llm.OpenText2Image(&cfg.Text2Image)
if err != nil { ... }
```

Adjust all constructor calls and type references to match the actual code.

- [ ] **Step 2: Commit (best-effort, will be refined in following tasks)**

```bash
git add internal/bootstrap/app.go
git commit -m "feat: create bootstrap/app.go single-function DI"
```

---

### Task 9: Update `cmd/main.go`

**Files:**
- Modify: `cmd/main.go`

Replace the entire `run()` function and main wiring to use `bootstrap.NewApp()`.

- [ ] **Step 1: Rewrite `cmd/main.go` imports and `run()`**

Remove old imports and change `run()`:

```go
package main

import (
	"context"
	"flag"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/bootstrap"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	pkglog "github.com/gonotelm-lab/gonotelm/pkg/log"
)

func main() {
	configPath := flag.String("config", "./etc/gonotelm.toml.tpl", "config file path")
	flag.Parse()

	initConfig(*configPath)
	initLogger()

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
	}
}

func initConfig(configPath string) {
	cfg, err := conf.Load(configPath)
	if err != nil {
		panic(err)
	}
	conf.SetGlobal(cfg)
}

func initLogger() {
	pkglog.Init()
	cfg := conf.Global()
	if cfg == nil {
		return
	}
	if err := pkglog.SetLevelText(cfg.Logging.Level); err != nil {
		panic(err)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := bootstrap.NewApp(ctx, conf.Global())
	if err != nil {
		return err
	}
	defer app.Close()

	app.Server.Run()
	return nil
}
```

Remove old imports: `"github.com/gonotelm-lab/gonotelm/internal/api"`, `"github.com/gonotelm-lab/gonotelm/internal/app/logic"`, `"github.com/gonotelm-lab/gonotelm/internal/infra"`, `"github.com/gonotelm-lab/gonotelm/internal/interfaces/event"`, `wire "github.com/gonotelm-lab/gonotelm/internal/wire"`.

- [ ] **Step 2: Commit**

```bash
git add cmd/main.go
git commit -m "refactor: simplify cmd/main.go with bootstrap.NewApp"
```

---

### Task 10: Update `conf/config.go` imports

**Files:**
- Modify: `internal/conf/config.go`

Update type import paths. The struct definition stays flat; only import paths change.

- [ ] **Step 1: Update imports in config.go**

Current imports that need changing:

```go
// OLD imports:
cache "github.com/gonotelm-lab/gonotelm/internal/infra/cache"
vecimpl "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/impl"
storageimpl "github.com/gonotelm-lab/gonotelm/internal/infra/storage/impl"
mqimpl "github.com/gonotelm-lab/gonotelm/internal/infra/mq/impl"
embedding "github.com/gonotelm-lab/gonotelm/internal/infra/llm/embedding"
rerank "github.com/gonotelm-lab/gonotelm/internal/infra/llm/rerank"
text2image "github.com/gonotelm-lab/gonotelm/internal/infra/llm/text2image"
chat "github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"

// NEW imports:
cache "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
vecimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"
storageimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
mqimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
embedding "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
rerank "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
text2image "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
chat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
```

Note: if `embedding`, `rerank`, `text2image`, `chat` config types are now all in the same `llm` package, those aliases will adjust accordingly. Read the actual `llm.go` to confirm which types are exported under which names.

- [ ] **Step 2: Commit**

```bash
git add internal/conf/config.go
git commit -m "refactor: update conf/config.go imports"
```

---

### Task 11: Update `api/server.go` — remove dead `infras` parameter

**Files:**
- Modify: `internal/api/server.go`

The `infras *infra.Instances` parameter is unused (dead code). Remove it.

- [ ] **Step 1: Update `NewServer` signature**

Change:

```go
func NewServer(
	logic *logic.Logic,
	infras *infra.Instances,
	wire *wire.Wire,
) *Server {
```

To:

```go
func NewServer(
	logic *logic.Logic,
	repos *app.Repos,
	bus *eventbus.EventBus,
	// ... fields previously accessed from wire.Wire
) *Server {
```

Read the full `internal/api/server.go` body. It accesses `wire.NotebookRepo`, `wire.SourceRepo`, `wire.SourceStorageRepo`, `wire.SourceDocRepo`, `wire.EventBus`, `wire.ChatRepo`, `wire.MessageRepo`, `wire.ContextMessageRepo`, `wire.StreamTaskRepo`, `wire.WaitGroup`, `wire.Gateway()`. Replace the `wire *wire.Wire` parameter with the specific types it actually needs, or create a smaller config struct.

Recommended approach: create a minimal `ServerDeps` struct:

```go
package api

type ServerDeps struct {
	NotebookRepo      notebookrepo.Repository
	SourceRepo        sourcerepo.Repository
	SourceStorageRepo sourcerepo.StorageRepository
	SourceDocRepo     sourcerepo.SourceDocRepository
	ChatRepo          chatrepo.Repository
	MessageRepo       chatrepo.MessageRepository
	ContextMessageRepo chatrepo.ContextMessageRepository
	StreamTaskRepo    chatrepo.StreamTaskRepository
	EventBus          eventbus.EventBus
	WaitGroup         *sync.WaitGroup
	Gateway           *gateway.Gateway
}

func NewServer(
	logic *logic.Logic,
	deps ServerDeps,
) *Server {
	// ... use deps.NotebookRepo instead of wire.NotebookRepo, etc.
}
```

This eliminates the `wire.Wire` dependency entirely.

- [ ] **Step 2: Update all handlers in server.go**

Replace every `wire.NotebookRepo` with `deps.NotebookRepo`, `wire.ChatRepo` with `deps.ChatRepo`, etc.

- [ ] **Step 3: Remove old imports**

Remove `"github.com/gonotelm-lab/gonotelm/internal/infra"` and `wire "github.com/gonotelm-lab/gonotelm/internal/wire"` imports.

- [ ] **Step 4: Commit**

```bash
git add internal/api/server.go
git commit -m "refactor: remove wire.Wire from api/server.go, use ServerDeps"
```

---

### Task 12: Update `app/logic/logic.go` and `app/logic/source/logic.go`

**Files:**
- Modify: `internal/app/logic/logic.go` — remove `infrastructures *infra.Instances`, accept specific types
- Modify: `internal/app/logic/source/logic.go` — remove `infras *infra.Instances`, accept `mqFactory *mq.MQ, redisClient redis.UniversalClient`

- [ ] **Step 1: Update `app/logic/logic.go`**

Change `MustNewLogic` signature from:

```go
func MustNewLogic(
	ctx context.Context,
	infrastructures *infra.Instances,
	objectStorage storage.Storage,
) *Logic {
```

To:

```go
func MustNewLogic(
	ctx context.Context,
	objectStorage storage.Storage,
	notebookStore database.NotebookStore,
	artifactTaskStore database.ArtifactTaskStore,
	sourceStore database.SourceStore,
	sourceDocStore vectordb.SourceDocStore,
	llmGateway *gateway.Gateway,
	embeddingGateway *embedding.Gateway,
	text2imageGateway *text2image.Gateway,
	mqFactory *mq.MQ,
	redisClient redis.UniversalClient,
) *Logic {
```

In the body, replace:
- `infrastructures.Dal.NotebookStore` → `notebookStore`
- `infrastructures.Dal.ArtifactTaskStore` → `artifactTaskStore`
- `infrastructures.Dal.SourceStore` → `sourceStore`
- `infrastructures.VectorDal.SourceDocStore` → `sourceDocStore`
- When passing to `sourcelogic.MustNewLogic`: replace `infrastructures` with `mqFactory, redisClient`
- **Remove** internal gateway creation lines: `gateway.MustNewGateway(conf.Global())`, `embedding.MustNewGateway(conf.Global())`, `text2image.MustNewGateway(conf.Global())` — these are now created in `bootstrap/app.go` and passed as parameters `llmGateway`, `embeddingGateway`, `text2imageGateway`.

- [ ] **Step 2: Update `app/logic/source/logic.go`**

Change `MustNewLogic` signature from:

```go
func MustNewLogic(
	rootCtx context.Context,
	infras *infra.Instances,
	objectStorage storage.Storage,
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
	llmGateway *gateway.Gateway,
	prompt *bizprompt.Prompt,
) *Logic {
```

To:

```go
func MustNewLogic(
	rootCtx context.Context,
	mqFactory *mq.MQ,
	redisClient redis.UniversalClient,
	objectStorage storage.Storage,
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
	llmGateway *gateway.Gateway,
	prompt *bizprompt.Prompt,
) *Logic {
```

In the body:
- `infras.MQ` → `mqFactory`
- `infras.Redis` → `redisClient`

- [ ] **Step 3: Update `bootstrap/app.go` to pass new parameters**

After Task 12 steps 1-2 are done, update `bootstrap/app.go` to match the new `MustNewLogic` signatures with explicit parameters.

- [ ] **Step 4: Commit**

```bash
git add internal/app/logic/logic.go internal/app/logic/source/logic.go internal/bootstrap/app.go
git commit -m "refactor: eliminate Infrastructures from logic constructors"
```

---

### Task 13: Global import path rewrite — remaining files

**Files to scan and update:**
All `.go` files outside `infrastructure/` that still reference old `internal/infra` or `internal/wire` paths.

Run a comprehensive search for remaining old import paths and fix them.

- [ ] **Step 1: Find remaining old imports**

```bash
rg "internal/infra" --no-heading internal/ | grep -v "internal/infrastructure" | grep -v "_test.go"
rg "internal/wire" --no-heading internal/ | grep -v "_test.go"
```

- [ ] **Step 2: Fix each remaining match**

For each file found, update the import path. Common files that will need changes:

| File | Old Import | New Import |
|------|-----------|------------|
| `internal/domain/agent/agent.go` | `infra/llm/chat` | `infrastructure/llm` |
| `internal/app/agent/agent.go` | `infra/llm/chat` | `infrastructure/llm` |
| `internal/app/model/*.go` | `infra/dal/schema` | `infrastructure/database/schema` |
| `internal/app/model/*.go` | `infra/cache/schema` | `infrastructure/cache/schema` |
| `internal/app/model/*.go` | `infra/vectordal/schema` | `infrastructure/vectordb/schema` |
| `internal/application/notebook/*.go` | `infra/dal/schema` | `infrastructure/database/schema` |
| `internal/application/source/*.go` | `infra/dal/schema` | `infrastructure/database/schema` |
| `internal/application/source/*.go` | `infra/vectordal/schema` | `infrastructure/vectordb/schema` |
| `internal/application/chat/*.go` | `infra/dal/schema` | `infrastructure/database/schema` |
| `internal/interfaces/event/*.go` | `internal/wire` | `internal/bootstrap` or specific types |

Use `rg` output to create an exhaustive list. Apply all replacements.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "refactor: global import path rewrite — eliminate old infra/wire imports"
```

---

### Task 14: Delete old directories and verify

**Files to delete:**
- `internal/infra/` (entire directory)
- `internal/wire/` (entire directory)
- Old `internal/infrastructure/` content that was moved into the new structure

Wait — the old `internal/infrastructure/` is the SAME directory we're populating. After Task 7, the files under `internal/infrastructure/repository/`, `eventbus/`, `adapter/` are already in place with updated imports. The old `internal/infra/` files still exist alongside. This task removes the stale duplicates.

- [ ] **Step 1: Delete old `internal/infra/`**

```bash
rm -rf internal/infra
```

- [ ] **Step 2: Delete old `internal/wire/`**

```bash
rm -rf internal/wire
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: zero errors.

- [ ] **Step 4: Vet**

```bash
go vet ./...
```

Expected: zero errors.

- [ ] **Step 5: Run all tests**

```bash
go test ./...
```

Expected: all tests pass. If any test fails, it's likely a test file that still references old import paths. Fix those test files (change import paths only, no test logic changes).

- [ ] **Step 6: Final search verification**

```bash
rg "internal/infra" $(go env GOMOD) --go
rg "internal/wire" $(go env GOMOD) --go
```

Expected: zero results for both.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor: delete old infra/ and wire/ directories"
```

---

### Task 15: Update `internal/interfaces/event/eventhandler.go`

**Files:**
- Modify: `internal/interfaces/event/eventhandler.go`

Currently this file likely has an `Init(ctx context.Context, wire *wire.Wire)` function (called from `cmd/main.go`). Event handler registration now moves into `bootstrap/app.go`.

- [ ] **Step 1: Check current eventhandler.go**

Read `internal/interfaces/event/eventhandler.go` and `internal/interfaces/event/` to see all files and the `Init()` function signature.

- [ ] **Step 2: Move event handler registration into bootstrap/app.go**

The event handler registration (MQ consumers, inner event consumers) that was in `interfaces/event/` should be called from `bootstrap/app.go` step 7. Update:

```go
// In bootstrap/app.go, after creating the event bus and before creating the server:
event.Init(ctx, bus, notebookLogic, srcLogic, ...)
```

The `Init` function signature should change from accepting `*wire.Wire` to accepting the specific types it needs (the EventBus and any handlers it registers).

- [ ] **Step 3: Commit**

```bash
git add internal/interfaces/event/ internal/bootstrap/app.go
git commit -m "refactor: move event handler registration to bootstrap"
```

---

### Final Verification

After all tasks complete, run:

```bash
go build ./...
go vet ./...
go test ./...
rg "internal/infra" --go && echo "FAIL: old infra imports remain" || echo "PASS"
rg "internal/wire" --go && echo "FAIL: old wire imports remain" || echo "PASS"
```

### Dead Code Sweep

Run a final dead code check after build passes to catch any remaining dead code (unused types, variables, functions revealed by `go vet` or staticcheck):

```bash
# Run staticcheck if available
staticcheck ./... 2>/dev/null || echo "staticcheck not installed"

# Check for any unused imports revealed during migration
go vet ./...

# Verify no dead provider configs were migrated
rg "ArkConfig|DashScopeConfig|GeminiConfig|OllamaConfig|QianfanConfig|TencentCloudConfig" --go && echo "WARNING: check if provider configs are still used"
```

Any dead code found should be removed before final merge.

All checks must pass.
