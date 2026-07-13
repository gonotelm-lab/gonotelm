.PHONY: gofmt
gofmt:
	go fmt ./...

.PHONY: run-worker
run-worker:
	@set -a && . ./.env && set +a && go run ./cmd/worker/main.go

.PHONY: run-gonotelm
run-gonotelm:
	@set -a && . ./.env && set +a && go run ./cmd/gonotelm/main.go
