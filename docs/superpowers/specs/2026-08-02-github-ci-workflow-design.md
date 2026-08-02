# GitHub CI Workflow Design

Date: 2026-08-02

## Goal

Add a GitHub Actions CI workflow for the gonotelm repo that runs, on every push and pull request:

1. golangci-lint
2. Go build
3. DB unit tests against a real PostgreSQL 18

## Constraints & Context

- Go module: `github.com/gonotelm-lab/gonotelm`, `go 1.25.4`.
- Migration file `migration/db/postgres18.sql` uses `uuidv7()` defaults, a PostgreSQL 18 built-in function. PG 16 (bundled on `ubuntu-24.04` runner) cannot run it; the `ubuntu-26.04` preview runner bundles PG 18.4, but we avoid preview images.
- DB tests (`internal/infrastructure/database/postgres/*`, `internal/infrastructure/repository/artifact_test.go`) use `pkg/sql/testsuite.NewTestGormDBFromEnv("pgsql")`, which creates a random test database, applies the migration, and drops it afterwards. Requires env vars `GONOTELM_DB_HOST`, `GONOTELM_DB_PORT`, `GONOTELM_DB_USER`, `GONOTELM_DB_PASS`, `GONOTELM_DB_DBNAME`.
- `internal/infrastructure/vectordb/milvus` tests self-skip when `GONOTELM_MILVUS_ADDR` is empty.
- `internal/infrastructure/cache/redis` tests have no skip guard and require a live Redis; excluded from CI test run.

## Chosen Approach

Single workflow file `.github/workflows/ci.yml` with three parallel jobs on `ubuntu-24.04`:

### Job: lint

- `actions/checkout@v4`
- `actions/setup-go@v5` with `go-version-file: go.mod`, caching
- `golangci/golangci-lint-action@v6`, pinned `version: v2.1.2`, `args: --timeout=5m`

### Job: build

- `actions/checkout@v4`
- `actions/setup-go@v5` with `go-version-file: go.mod`, caching
- `go build ./...`

### Job: test

- `services.postgres`: `postgres:18` image, user/pass/db `postgres`, port 5432, `pg_isready` health check
- Job env: `GONOTELM_DB_HOST=127.0.0.1`, `GONOTELM_DB_PORT=5432`, `GONOTELM_DB_USER=postgres`, `GONOTELM_DB_PASS=postgres`, `GONOTELM_DB_DBNAME=postgres`
- `go test $(go list ./... | grep -v /internal/infrastructure/cache/redis)`

## Non-Goals

- No Docker image build/push (no Dockerfile in repo).
- No redis/milvus/kafka/minio services in CI.
- No golangci-lint config file (default linters; can be added later if defaults are noisy).
