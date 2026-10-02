# deploy/dev —— 本地开发基础设施

只包含 GoNoteLM 需要的中间件，`notelm` / `worker` / `sourcejob` 仍在宿主机 `go run`（根目录 `run-*`），
所以这里发布的宿主机端口必须和 `.env` 里 `GONOTELM_*` 指向的地址一致。

## 快速开始

```bash
make dev-up        # 起默认那套中间件 + jaeger（= make -C deploy/dev up）
make dev-up-all    # 再带上 flow-web 控制台
make dev-ps        # 看状态（含跑完即退出的 init 容器）
make dev-down      # 停止，保留数据卷；down-v 连数据卷一起删
```

`up` 默认带 `--profile trace` 起 **jaeger**：app 侧 `trace.Init` 是无条件调用的，endpoint 默认
`127.0.0.1:4317`（`etc/*.toml.tpl` 里 `OTEL_TRACE_ENDPOINT` 的默认值）。jaeger 不在时 exporter 会
一直重试导出，日志被 `[otel] error` 刷屏。只想要中间件、不要 jaeger 时：

```bash
make dev-up-notrace   # = make -C deploy/dev up-notrace
```

变量默认取仓库根目录的 `.env`（也就是 app 用的那份），优先级：**shell 里 export 的 > `--env-file` 指定的**。
所以临时换端口不用改文件：`GONOTELM_DB_PORT=15432 make dev-up`。
必需变量（缺失时 compose 直接报错）：`GONOTELM_DB_PASSWORD`、`GONOTELM_OLAP_DB_PASSWORD`、
`GONOTELM_MINIO_SECRET_KEY`、`GONOTELM_KAFKA_PASSWORD`。

## 服务与端口

| 服务 | 宿主端口 | app 侧对应变量 |
| --- | --- | --- |
| postgres | 5432 | `GONOTELM_DB_PORT` |
| redis | 7542 | `GONOTELM_REDIS_ADDRS` |
| clickhouse | 9009（原生）/ 8123（HTTP） | `GONOTELM_OLAP_DB_PORT` |
| kafka | 9094（EXTERNAL，SASL）/ 9092（容器内） | `GONOTELM_KAFKA_BROKER` |
| minio | 9000 / 9001（控制台） | `GONOTELM_MINIO_ENDPOINT` |
| etcd | 不发布 | 只给 milvus 用 |
| milvus | 19530 / 9091（健康检查） | `GONOTELM_MILVUS_ADDR` |
| flow-server | 7091（gRPC）/ 7090（HTTP） | `GONOTELM_FLOW_ADDR` |
| flow-web（profile `ui`，`up-all` 带） | 7089 | 无 |
| jaeger（profile `trace`，`up` 默认带） | 4317 / 4318 / 16686 | `OTEL_TRACE_ENDPOINT` |

一次性容器跑完即退出（`Exited (0)` 正常）：`kafka-init` 建 topic、`etcd-init` 修 etcd 数据目录属主、
`minio-init` 建 bucket + 建桶凭据。

### MinIO 的桶凭据

`minio-init` 跑的是仓库里的 `migration/storage/minio.sh`（挂载进容器用 `/bin/sh` 执行，宿主机也能直接跑），
用 root（`GONOTELM_DEV_MINIO_ROOT_USER` / `GONOTELM_DEV_MINIO_ROOT_PASSWORD`，默认 `minioadmin`）做管理操作：

* 建 `GONOTELM_MINIO_BUCKET`（默认 `gonotelm`），已存在则原样保留数据；
* 桶访问策略设为 private，匿名不能读也不能写；
* 用 `GONOTELM_MINIO_ACCESS_KEY` / `GONOTELM_MINIO_SECRET_KEY` 建桶自己的凭据，只授予该桶读写
  （策略 `gonotelm-rw`），并摘掉可能存在的全局 `readwrite` / `writeonly` / `readonly`。

脚本可重复执行：不会清空桶，改完 `.env` 里的 AK/SK 重跑即轮换密钥。宿主机手动执行：

```bash
set -a && . ./.env && set +a && migration/storage/minio.sh
```

注意 AK 必须是**未被占用**的：如果它已经是某个 service account（MinIO 控制台建的 app 凭据常见这种），
`mc admin user add` 只会回一句 `Credential is not allowed to be same as admin access key`。脚本会提前拦住并提示，
此时要么换一个 `GONOTELM_MINIO_ACCESS_KEY`，要么先删掉旧的：

```bash
mc admin user svcacct rm <alias> <旧 AK>   # 之后重跑脚本即可
```

### 谁用哪份 MinIO 凭据

| 使用者 | 桶 | 凭据 |
| --- | --- | --- |
| app（`notelm` / `worker` / `sourcejob`） | `gonotelm` | `GONOTELM_MINIO_ACCESS_KEY` / `SECRET_KEY`，桶级策略 `gonotelm-rw` |
| `minio-init` | 建桶、建桶用户 | `GONOTELM_DEV_MINIO_ROOT_*`（默认 `minioadmin`） |
| milvus | `a-bucket`（Milvus 自己的存储后端） | `GONOTELM_DEV_MINIO_ROOT_*` |

milvus 不共用 app 的桶凭据：那份凭据只授权 `gonotelm`，milvus 用它读不了 `a-bucket`。
所以 compose 里 milvus 直接走 root；生产环境应换成 milvus 专属用户 + `a-bucket` 策略。

## 首次在新数据卷上初始化

`up` 起来的是一套空中间件，schema 还要跑一次项目自带的迁移：

```bash
make dev-migrate
```

* postgres：`go run ./cmd/migrate`（建 `gonotelm` 库 + goose 增量迁移 `migration/db/postgres18`，
  可重复执行）。`-baseline` 兼容引入 goose 之前用 psql 建的旧库：会把已存在的旧 schema
  标记为迁移 0001 已应用，不会重复建表。
* clickhouse / milvus：`migration/db/clickhouse/unclustered/0001.up.sql`、`migration/db/milvus/milvus-*.sh`，
  **脚本非幂等，只在全新数据卷上跑一次**。

`flowdb` 由 flow-server 的 `FLOW_DB_AUTO_INIT` 自己创建。
**接上已有数据目录时**，postgres 可以直接跑（会自动 baseline），clickhouse/milvus 的脚本不要重复执行。

## 接上已有的本地数据目录

默认每个服务用自己的具名卷。想复用已有数据，把数据目录覆盖成宿主路径即可（示例见 `.env.example`）：

```bash
GONOTELM_DEV_PG_DATA=$HOME/pgsql-data \
GONOTELM_DEV_MINIO_DATA=$HOME/minio-storage/data \
GONOTELM_DEV_MILVUS_DATA=$HOME/milvus-data/volumes/milvus \
GONOTELM_DEV_ETCD_DATA=$HOME/etcd-data GONOTELM_DEV_ETCD_USER=1000:1000 \
  make dev-up
```

* `GONOTELM_DEV_ETCD_USER` 必须和 etcd 数据目录的属主一致（目录属主是登录用户时填 `1000:1000`），
  否则 etcd 起不来、milvus 会一直等它。`etcd-init` 只在属主是 root 时 chown，不会动你自己的目录。
* 这些目录不再由本 compose 独占，别同时起第二套中间件读写同一目录。

## 和机器上已有的中间件同时运行

默认端口和 app 的 `.env` 一致，如果已被占用，两套不能同时启动（容器名不冲突，端口会撞）。并存就同时改端口：

```bash
GONOTELM_DB_PORT=15432 GONOTELM_REDIS_PORT=17542 GONOTELM_OLAP_DB_PORT=19009 \
GONOTELM_OLAP_HTTP_PORT=18123 GONOTELM_KAFKA_PORT=19092 GONOTELM_KAFKA_EXTERNAL_PORT=19094 \
GONOTELM_MINIO_PORT=19000 GONOTELM_MINIO_CONSOLE_PORT=19001 \
GONOTELM_MILVUS_PORT=29530 GONOTELM_MILVUS_HEALTH_PORT=29091 \
GONOTELM_FLOW_PORT=27091 GONOTELM_FLOW_HTTP_PORT=27090 \
  make dev-up
```

app 侧的 `.env` 也要跟着改（或用同一批变量启动 app）。

## 说明

* 不含 opensandbox：它是宿主机进程（`opensandbox-server`，配置见 `etc/worker.toml.tpl` 的
  `[sandbox.opensandbox]`），需要 docker socket 和宿主路径挂载，不属于常驻中间件。
* `imgproxy` / `s1-gate` / `elasticsearch` / `mysql` 项目里没有引用，没有放进来。
