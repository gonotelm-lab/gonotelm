deployEnv = "dev"

[database]
type = "postgres"
host = "${GONOTELM_DB_HOST:-127.0.0.1}"
port = ${GONOTELM_DB_PORT:-5432}
user = "${GONOTELM_DB_USER:-postgres}"
password = "${GONOTELM_DB_PASSWORD:-postgres}"
dbName = "${GONOTELM_DB_NAME:-gonotelm}"


[vectorDb]
type = "milvus"

[vectorDb.milvus]
addr = "${GONOTELM_MILVUS_ADDR:-127.0.0.1:19530}"
username = "${GONOTELM_MILVUS_USERNAME:-}"
password = "${GONOTELM_MILVUS_PASSWORD:-}"
dbName = "${GONOTELM_MILVUS_DB_NAME:-gonotelm}"
dialTimeout = "${GONOTELM_MILVUS_DIAL_TIMEOUT:-10s}"

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


[logging]
level = "${GONOTELM_LOG_LEVEL:-debug}"

[logic]

[logic.studio.mindmap]
maxRound = ${GONOTELM_LOGIC_STUDIO_MINDMAP_MAX_ROUND:-50}
modelProvider = "${GONOTELM_LOGIC_STUDIO_MINDMAP_PROVIDER:-deepseek}"
model = "${GONOTELM_LOGIC_STUDIO_MINDMAP_MODEL:-deepseek-v4-flash}"

[logic.studio.report]
maxRound = ${GONOTELM_LOGIC_STUDIO_REPORT_MAX_ROUND:-50}
modelProvider = "${GONOTELM_LOGIC_STUDIO_REPORT_PROVIDER:-deepseek}"
model = "${GONOTELM_LOGIC_STUDIO_REPORT_MODEL:-deepseek-v4-flash}"

[logic.studio.infoGraphic]
maxRound = ${GONOTELM_LOGIC_STUDIO_INFOGRAPHIC_MAX_ROUND:-50}
modelProvider = "${GONOTELM_LOGIC_STUDIO_INFOGRAPHIC_PROVIDER:-deepseek}"
model = "${GONOTELM_LOGIC_STUDIO_INFOGRAPHIC_MODEL:-deepseek-v4-flash}"
imageModelProvider = "${GONOTELM_LOGIC_STUDIO_INFOGRAPHIC_IMAGE_MODEL_PROVIDER:-dashscope}"
imageModel = "${GONOTELM_LOGIC_STUDIO_INFOGRAPHIC_IMAGE_MODEL:-qwen-image-2.0-pro}"






[embedding]
type = "${GONOTELM_EMBEDDING_TYPE:-dashscope}"
batchSize = ${GONOTELM_EMBEDDING_BATCH_SIZE:-10}
maxConcurrency = ${GONOTELM_EMBEDDING_MAX_CONCURRENCY:-4}

[embedding.dashscope]
apiKey = "${GONOTELM_DASHSCOPE_API_KEY:-}"
model = "${GONOTELM_DASHSCOPE_MODEL:-text-embedding-v4}"
timeout = "${GONOTELM_DASHSCOPE_TIMEOUT:-30s}"
dimensions = ${GONOTELM_DASHSCOPE_DIMENSIONS:-1024}

[embedding.openai]
apiKey = "${GONOTELM_OPENAI_API_KEY:-}"
model = "${GONOTELM_OPENAI_MODEL:-text-embedding-3-small}"
timeout = "${GONOTELM_OPENAI_TIMEOUT:-30s}"
encodingFormat = "${GONOTELM_OPENAI_ENCODING_FORMAT:-float}"
dimensions = ${GONOTELM_OPENAI_DIMENSIONS:-1024}
user = "${GONOTELM_OPENAI_USER:-}"
byAzure = ${GONOTELM_OPENAI_BY_AZURE:-false}
baseUrl = "${GONOTELM_OPENAI_BASE_URL:-}"
apiVersion = "${GONOTELM_OPENAI_API_VERSION:-}"

[provider]

[provider.deepseek]
apiKey = "${GONOTELM_DEEPSEEK_API_KEY}"
timeout = "${GONOTELM_DEEPSEEK_TIMEOUT:-5m}"
baseUrl = "${GONOTELM_DEEPSEEK_BASE_URL:-https://api.deepseek.com}"
model = "${GONOTELM_DEEPSEEK_MODEL:-deepseek-v4-flash}"
maxTokens = ${GONOTELM_DEEPSEEK_MAX_TOKENS:-16384}
thinkingEnabled = false

[provider.qwen]
apiKey = "${GONOTELM_DASHSCOPE_API_KEY:-}"
baseUrl = "${GONOTELM_QWEN_BASE_URL:-https://dashscope.aliyuncs.com/compatible-mode/v1}"
model = "${GONOTELM_QWEN_MODEL:-glm-5.1}"
timeout = "${GONOTELM_QWEN_TIMEOUT:-5m}"
maxTokens = ${GONOTELM_QWEN_MAX_TOKENS:-16384}
temperature = ${GONOTELM_QWEN_TEMPERATURE:-1.0}
topP = ${GONOTELM_QWEN_TOP_P:-1.0}
enableThinking = ${GONOTELM_QWEN_ENABLE_THINKING:-false}

[text2image]
type = "${GONOTELM_TEXT2IMAGE_TYPE:-dashscope}"

[text2image.dashscope]
apiKey = "${GONOTELM_DASHSCOPE_API_KEY:-}"
baseUrl = "${GONOTELM_TEXT2IMAGE_DASHSCOPE_BASE_URL:-https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation}"
model = "${GONOTELM_TEXT2IMAGE_DASHSCOPE_MODEL:-qwen-image-2.0-pro}"
timeout = "${GONOTELM_TEXT2IMAGE_DASHSCOPE_TIMEOUT:-1h}"

[flow]
addr        = "${GONOTELM_FLOW_ADDR:-flow.example:9443}"
namespace   = "${GONOTELM_FLOW_NAMESPACE:-gonotelm}"
maxRetry    = ${GONOTELM_FLOW_MAX_RETRY:-3}
dialTimeout = "${GONOTELM_FLOW_DIAL_TIMEOUT:-5s}"

[worker]
maxConcurrency  = ${GONOTELM_WORKER_MAX_CONCURRENCY:-4}
heartbeat       = "${GONOTELM_WORKER_HEARTBEAT:-5s}"
