# GoNoteLM 本地开发基础设施（只起中间件，app 在宿主机 go run）
# up / up-notrace / up-all / down / down-v / ps / logs / config / migrate；
# 根目录也有同名快捷命令：make dev-up、make dev-down ...
# 变量取自仓库根目录的 .env（ENV_FILE=... 可换），shell 里 export 的优先级更高
COMPOSE ?= docker compose
ENV_FILE ?= ../../.env
COMPOSE_ENV := --env-file $(ENV_FILE)
# app 侧 trace.Init 是无条件调用的，endpoint 默认 127.0.0.1:4317（etc/*.toml.tpl），
# 所以 up 默认就带上 jaeger（profile trace），否则 exporter 会一直重试并刷 otel 报错日志
COMPOSE_TRACE := $(COMPOSE) $(COMPOSE_ENV) --profile trace
# down/ps/logs 要带 --profile full，否则 flow-web / jaeger 不会被处理
COMPOSE_ALL := $(COMPOSE) $(COMPOSE_ENV) --profile full
LOAD_ENV = set -a && . $(ENV_FILE) && set +a

.PHONY: up up-notrace up-all down down-v ps logs config migrate migrate-postgres migrate-clickhouse migrate-milvus

up:
	$(COMPOSE_TRACE) up -d

# 只要中间件、不要 jaeger（CI、或者本地不跑 app 时用）
up-notrace:
	$(COMPOSE) $(COMPOSE_ENV) up -d

up-all:
	$(COMPOSE_ALL) up -d

down:
	$(COMPOSE_ALL) down

down-v:
	$(COMPOSE_ALL) down -v

ps:
	$(COMPOSE_ALL) ps --all

logs:
	$(COMPOSE_ALL) logs -f --tail=100

config:
	$(COMPOSE) $(COMPOSE_ENV) config

# clickhouse / milvus 只在全新数据卷上跑一次，脚本非幂等；postgres 走 goose，可重复执行。
migrate: migrate-postgres migrate-clickhouse migrate-milvus

migrate-postgres:
	@$(LOAD_ENV) && cd ../.. && go run ./cmd/migrate -baseline

# 0001.up.sql 的建表语句不带库名，必须先把默认库指定成 GONOTELM_OLAP_DB_NAME
migrate-clickhouse:
	@$(LOAD_ENV) && db="$${GONOTELM_OLAP_DB_NAME:-gonotelm}" && \
		$(COMPOSE) $(COMPOSE_ENV) exec -T clickhouse \
			clickhouse-client --query "CREATE DATABASE IF NOT EXISTS $$db" && \
		$(COMPOSE) $(COMPOSE_ENV) exec -T clickhouse \
			clickhouse-client --database "$$db" --multiquery \
			< ../../migration/db/clickhouse/unclustered/0001.up.sql

# milvus 脚本用 curl 访问宿主端口，凭证取自 .env
migrate-milvus:
	@$(LOAD_ENV) && for f in ../../migration/db/milvus/milvus-*.sh; do \
		echo "==> $$f"; \
		sh "$$f" || exit 1; \
	done
