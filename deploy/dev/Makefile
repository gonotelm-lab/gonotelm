# GoNoteLM 本地开发基础设施（只起中间件，app 在宿主机 go run）
# up / up-all / down / down-v / ps / logs / config / migrate；
# 根目录也有同名快捷命令：make dev-up、make dev-down ...
# 变量取自仓库根目录的 .env（ENV_FILE=... 可换），shell 里 export 的优先级更高
COMPOSE ?= docker compose
ENV_FILE ?= ../../.env
COMPOSE_ENV := --env-file $(ENV_FILE)
# down/ps/logs 要带 --profile full，否则 flow-web / jaeger 不会被处理
COMPOSE_ALL := $(COMPOSE) $(COMPOSE_ENV) --profile full
LOAD_ENV = set -a && . $(ENV_FILE) && set +a

.PHONY: up up-all down down-v ps logs config migrate migrate-postgres migrate-clickhouse migrate-milvus

up:
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

# 只在全新数据卷上跑一次，脚本非幂等
migrate: migrate-postgres migrate-clickhouse migrate-milvus

migrate-postgres:
	@$(LOAD_ENV) && $(COMPOSE) $(COMPOSE_ENV) exec -T postgres \
		psql -U "$${GONOTELM_DB_USER:-postgres}" -d postgres \
		< ../../migration/db/postgres18/0001.sql

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
