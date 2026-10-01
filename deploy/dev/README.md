# deploy/dev —— 本地开发基础设施

只包含 GoNoteLM 需要的中间件，`notelm` / `worker` / `sourcejob` 仍在宿主机 `go run`（根目录 `run-*`），
所以这里发布的宿主机端口必须和 `.env` 里 `GONOTELM_*` 指向的地址一致。

## 快速开始

```bash
make dev-up        # 起默认那套中间件（= make -C deploy/dev up）
make dev-up-all    # 再带上 flow-web 控制台和 jaeger
make dev-ps        # 看状态（含跑完即退出的 init 容器）
make dev-down      # 停止，保留数据卷；down-v 连数据卷一起删
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
| flow-web（profile `ui`） | 7089 | 无 |
| jaeger（profile `trace`） | 4317 / 4318 / 16686 | `OTEL_TRACE_ENDPOINT` |

一次性容器跑完即退出（`Exited (0)` 正常）：`kafka-init` 建 topic、`minio-init` 建 bucket、
`etcd-init` 修 etcd 数据目录属主。

## 首次在新数据卷上初始化

`up` 起来的是一套空中间件，schema 还要跑一次项目自带的迁移（**只在全新数据卷上执行，脚本非幂等**）：

```bash
make dev-migrate
```

依次执行 `migration/db/postgres18/0001.sql`（自建 `gonotelm` 库和表）、
`migration/db/clickhouse/unclustered/0001.up.sql`、`migration/db/milvus/milvus-*.sh`；
`flowdb` 由 flow-server 的 `FLOW_DB_AUTO_INIT` 自己创建。
**接上已有数据目录时不要跑它**，库/表/collection 都已经存在。

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
