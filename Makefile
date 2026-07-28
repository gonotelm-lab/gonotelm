.PHONY: gofmt
gofmt:
	go fmt ./...

.PHONY: run-worker
run-worker:
	@set -a && . ./.env && set +a && go run ./cmd/worker/main.go

.PHONY: run-sourcejob
run-sourcejob:
	@set -a && . ./.env && set +a && go run ./cmd/sourcejob/main.go

.PHONY: run-notelm
run-notelm:
	@set -a && . ./.env && set +a && go run ./cmd/notelm/main.go
