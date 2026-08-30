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
