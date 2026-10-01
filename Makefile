.PHONY: gofmt
gofmt:
	go fmt ./...

TEST_PKGS := \
	./internal/infrastructure/database/postgres/... \
	./internal/infrastructure/cache/redis/... \
	./internal/infrastructure/olap/...

TEST_GCFLAGS := all=-l

.PHONY: test
test:
	go test -gcflags="$(TEST_GCFLAGS)" -coverprofile=coverage.out $(TEST_PKGS)

.PHONY: test-coverage
test-coverage:
	@stamp=$$(date +%Y%m%d-%H%M%S); \
	out=/tmp/gonotelm-coverage-$$stamp.out; \
	html=/tmp/gonotelm-coverage-$$stamp.html; \
	go test -gcflags="$(TEST_GCFLAGS)" -coverprofile=$$out $(TEST_PKGS) && \
	go tool cover -html=$$out -o $$html && \
	rm -f $$out && \
	echo "coverage report: $$html"

.PHONY: clean-test
clean-test:
	rm -f coverage.out

.PHONY: run-worker
run-worker:
	@set -a && . ./.env && set +a && go run ./cmd/worker/main.go

.PHONY: run-sourcejob
run-sourcejob:
	@set -a && . ./.env && set +a && go run ./cmd/sourcejob/main.go

.PHONY: run-notelm
run-notelm:
	@set -a && . ./.env && set +a && go run ./cmd/notelm/main.go

# deploy/dev 中间件（app 用上面的 run-* 跑在宿主机）：dev-up / dev-up-all / dev-down / dev-down-v /
# dev-ps / dev-logs / dev-migrate；变量取 ./.env（ENV_FILE=... 可换），细节见 deploy/dev/README.md
DEV_DIR := deploy/dev
DEV_MAKE = $(MAKE) --no-print-directory -C $(DEV_DIR)

.PHONY: dev-up
dev-up:
	@$(DEV_MAKE) up

.PHONY: dev-up-all
dev-up-all:
	@$(DEV_MAKE) up-all

.PHONY: dev-down
dev-down:
	@$(DEV_MAKE) down

.PHONY: dev-down-v
dev-down-v:
	@$(DEV_MAKE) down-v

.PHONY: dev-ps
dev-ps:
	@$(DEV_MAKE) ps

.PHONY: dev-logs
dev-logs:
	@$(DEV_MAKE) logs

.PHONY: dev-migrate
dev-migrate:
	@$(DEV_MAKE) migrate
