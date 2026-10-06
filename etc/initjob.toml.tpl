deployEnv = "dev"

[database]
type = "postgres"
host = "${GONOTELM_DB_HOST:-127.0.0.1}"
port = ${GONOTELM_DB_PORT:-5432}
user = "${GONOTELM_DB_USER:-postgres}"
password = "${GONOTELM_DB_PASSWORD:-postgres}"
dbName = "${GONOTELM_DB_NAME:-gonotelm}"

[redis]
addrs = ${GONOTELM_REDIS_ADDRS:-['127.0.0.1:7542']}
username = "${GONOTELM_REDIS_USERNAME:-}"
password = "${GONOTELM_REDIS_PASSWORD:-}"

[storage]
type = "minio"

[storage.minio]
endpoint = "${GONOTELM_MINIO_ENDPOINT:-127.0.0.1:9000}"
accessKey = "${GONOTELM_MINIO_ACCESS_KEY:-minio}"
secretKey = "${GONOTELM_MINIO_SECRET_KEY:-minio}"
bucket = "gonotelm"
region = "${GONOTELM_MINIO_REGION:-us-east-1}"
secure = ${GONOTELM_MINIO_SECURE:-false}
presignExpiry = "${GONOTELM_MINIO_PRESIGN_EXPIRY:-15m}"
publicBucket = "${GONOTELM_PUBLIC_MINIO_BUCKET:-gonotelm-public}"
publicAccessKey = "${GONOTELM_PUBLIC_MINIO_ACCESS_KEY:-}"
publicSecretKey = "${GONOTELM_PUBLIC_MINIO_SECRET_KEY:-}"
publicBaseURL = "${GONOTELM_PUBLIC_MINIO_BASE_URL:-http://127.0.0.1:9000}"

[logging]
level = "${GONOTELM_LOG_LEVEL:-debug}"

[initJob]
mode = "${GONOTELM_INITJOB_MODE:-strict}"
forceTasks = "${GONOTELM_INITJOB_FORCE_TASKS:-}"

[otelTrace]
name = "initjob"
endpoint = "${OTEL_TRACE_ENDPOINT:-127.0.0.1:4317}"
exporter = "grpc"
