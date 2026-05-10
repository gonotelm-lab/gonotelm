deployEnv = "dev"

[api]
port = 7099
exitWaitTimeout = "${ENV_GONOTELM_API_EXIT_WAIT_TIMEOUT:-10s}"

[database]
type = "postgres"
host = "${ENV_GONOTELM_DB_HOST:-127.0.0.1}"
port = ${ENV_GONOTELM_DB_PORT:-5432}
user = "${ENV_GONOTELM_DB_USER:-postgres}"
password = "${ENV_GONOTELM_DB_PASSWORD:-postgres}"
dbName = "${ENV_GONOTELM_DB_NAME:-gonotelm}"

[redis]
addrs = ${ENV_GONOTELM_REDIS_ADDRS:-['127.0.0.1:7542']}
username = "${ENV_GONOTELM_REDIS_USERNAME:-}"
password = "${ENV_GONOTELM_REDIS_PASSWORD:-}"

[vectorDb]
type = "milvus"

[vectorDb.milvus]
addr = "${ENV_GONOTELM_MILVUS_ADDR:-127.0.0.1:19530}"
username = "${ENV_GONOTELM_MILVUS_USERNAME:-}"
password = "${ENV_GONOTELM_MILVUS_PASSWORD:-}"
dbName = "${ENV_GONOTELM_MILVUS_DB_NAME:-gonotelm}"
dialTimeout = "${ENV_GONOTELM_MILVUS_DIAL_TIMEOUT:-10s}"

[storage]
type = "minio"

[storage.minio]
endpoint = "${ENV_GONOTELM_MINIO_ENDPOINT:-127.0.0.1:9000}"
accessKey = "${ENV_GONOTELM_MINIO_ACCESS_KEY:-minio}"
secretKey = "${ENV_GONOTELM_MINIO_SECRET_KEY:-minio}"
bucket = "gonotelm"
region = "${ENV_GONOTELM_MINIO_REGION:-us-east-1}"
secure = ${ENV_GONOTELM_MINIO_SECURE:-false}
presignExpiry = "${ENV_GONOTELM_MINIO_PRESIGN_EXPIRY:-15m}"

[msgQueue]
type = "kafka"

[msgQueue.kafka]
brokers = ["${ENV_GONOTELM_KAFKA_BROKER:-127.0.0.1:9094}"]
username = "${ENV_GONOTELM_KAFKA_USERNAME:-kafka}"
password = "${ENV_GONOTELM_KAFKA_PASSWORD:-kafka}"
consumerQueueCapacity = ${ENV_GONOTELM_KAFKA_CONSUMER_QUEUE_CAPACITY:-100}
consumerCommitInterval = "${ENV_GONOTELM_KAFKA_CONSUMER_COMMIT_INTERVAL:-0s}"

[logging]
level = "${ENV_GONOTELM_LOG_LEVEL:-debug}"

[chunking]
size = ${ENV_GONOTELM_CHUNKING_SIZE:-500}
overlapSize = ${ENV_GONOTELM_CHUNKING_OVERLAP_SIZE:-75}

[embedding]
type = "${ENV_GONOTELM_EMBEDDING_TYPE:-dashscope}"
batchSize = ${ENV_GONOTELM_EMBEDDING_BATCH_SIZE:-10}
maxConcurrency = ${ENV_GONOTELM_EMBEDDING_MAX_CONCURRENCY:-4}

[embedding.ark]
apiKey = "${ENV_GONOTELM_ARK_API_KEY:-}"
accessKey = "${ENV_GONOTELM_ARK_ACCESS_KEY:-}"
secretKey = "${ENV_GONOTELM_ARK_SECRET_KEY:-}"
baseUrl = "${ENV_GONOTELM_ARK_BASE_URL:-https://ark.cn-beijing.volces.com/api/v3}"
region = "${ENV_GONOTELM_ARK_REGION:-cn-beijing}"
model = "${ENV_GONOTELM_ARK_MODEL:-}"
apiType = "${ENV_GONOTELM_ARK_API_TYPE:-text_api}"
timeout = "${ENV_GONOTELM_ARK_TIMEOUT:-10m}"
retryTimes = ${ENV_GONOTELM_ARK_RETRY_TIMES:-2}
maxConcurrentRequests = ${ENV_GONOTELM_ARK_MAX_CONCURRENT_REQUESTS:-5}

[embedding.dashscope]
apiKey = "${ENV_GONOTELM_DASHSCOPE_API_KEY:-}"
model = "${ENV_GONOTELM_DASHSCOPE_MODEL:-text-embedding-v4}"
timeout = "${ENV_GONOTELM_DASHSCOPE_TIMEOUT:-30s}"
dimensions = ${ENV_GONOTELM_DASHSCOPE_DIMENSIONS:-1024}

[embedding.gemini]
apiKey = "${ENV_GONOTELM_GEMINI_API_KEY:-}"
backend = "${ENV_GONOTELM_GEMINI_BACKEND:-gemini_api}"
project = "${ENV_GONOTELM_GEMINI_PROJECT:-}"
location = "${ENV_GONOTELM_GEMINI_LOCATION:-}"
model = "${ENV_GONOTELM_GEMINI_MODEL:-gemini-embedding-001}"
taskType = "${ENV_GONOTELM_GEMINI_TASK_TYPE:-RETRIEVAL_DOCUMENT}"
title = "${ENV_GONOTELM_GEMINI_TITLE:-}"
mimeType = "${ENV_GONOTELM_GEMINI_MIME_TYPE:-text/plain}"
autoTruncate = ${ENV_GONOTELM_GEMINI_AUTO_TRUNCATE:-true}

[embedding.ollama]
baseUrl = "${ENV_GONOTELM_OLLAMA_BASE_URL:-http://localhost:11434}"
model = "${ENV_GONOTELM_OLLAMA_MODEL:-bge-m3}"
timeout = "${ENV_GONOTELM_OLLAMA_TIMEOUT:-30s}"
truncate = ${ENV_GONOTELM_OLLAMA_TRUNCATE:-false}
keepAlive = "${ENV_GONOTELM_OLLAMA_KEEP_ALIVE:-5m}"

[embedding.openai]
apiKey = "${ENV_GONOTELM_OPENAI_API_KEY:-}"
model = "${ENV_GONOTELM_OPENAI_MODEL:-text-embedding-3-small}"
timeout = "${ENV_GONOTELM_OPENAI_TIMEOUT:-30s}"
encodingFormat = "${ENV_GONOTELM_OPENAI_ENCODING_FORMAT:-float}"
dimensions = ${ENV_GONOTELM_OPENAI_DIMENSIONS:-1536}
user = "${ENV_GONOTELM_OPENAI_USER:-}"
byAzure = ${ENV_GONOTELM_OPENAI_BY_AZURE:-false}
baseUrl = "${ENV_GONOTELM_OPENAI_BASE_URL:-}"
apiVersion = "${ENV_GONOTELM_OPENAI_API_VERSION:-}"

[embedding.qianfan]
ak = "${ENV_GONOTELM_QIANFAN_AK:-}"
sk = "${ENV_GONOTELM_QIANFAN_SK:-}"
accessKey = "${ENV_GONOTELM_QIANFAN_ACCESS_KEY:-}"
secretKey = "${ENV_GONOTELM_QIANFAN_SECRET_KEY:-}"
accessToken = "${ENV_GONOTELM_QIANFAN_ACCESS_TOKEN:-}"
bearerToken = "${ENV_GONOTELM_QIANFAN_BEARER_TOKEN:-}"
model = "${ENV_GONOTELM_QIANFAN_MODEL:-Embedding-V1}"

[embedding.tencentcloud]
secretId = "${ENV_GONOTELM_TENCENTCLOUD_SECRET_ID:-}"
secretKey = "${ENV_GONOTELM_TENCENTCLOUD_SECRET_KEY:-}"
region = "${ENV_GONOTELM_TENCENTCLOUD_REGION:-ap-guangzhou}"

[chatModel]
type = "qwen"

[chatModel.deepseek]
apiKey = "${ENV_GONOTELM_DEEPSEEK_API_KEY}"
timeout = "${ENV_GONOTELM_DEEPSEEK_TIMEOUT:-5m}"
baseUrl = "${ENV_GONOTELM_DEEPSEEK_BASE_URL:-https://api.deepseek.com}"
model = "${ENV_GONOTELM_DEEPSEEK_MODEL:-deepseek-v4-flash}"
maxTokens = ${ENV_GONOTELM_DEEPSEEK_MAX_TOKENS:-8192}
thinkingEnabled = true

[chatModel.openai]
apiKey = "${ENV_GONOTELM_OPENAI_API_KEY:-}"
baseUrl = "${ENV_GONOTELM_OPENAI_BASE_URL:-https://api.openai.com/v1}"
model = "${ENV_GONOTELM_OPENAI_MODEL:-gpt-4o-mini}"
timeout = "${ENV_GONOTELM_OPENAI_TIMEOUT:-5m}"
maxTokens = ${ENV_GONOTELM_OPENAI_MAX_TOKENS:-8192}
temperature = ${ENV_GONOTELM_OPENAI_TEMPERATURE:-1.0}
reasoningEffort = "${ENV_GONOTELM_OPENAI_REASONING_EFFORT:-medium}"

[chatModel.qwen]
apiKey = "${ENV_GONOTELM_DASHSCOPE_API_KEY:-}"
baseUrl = "${ENV_GONOTELM_QWEN_BASE_URL:-https://dashscope.aliyuncs.com/compatible-mode/v1}"
model = "${ENV_GONOTELM_QWEN_MODEL:-qwen3.6-plus}"
timeout = "${ENV_GONOTELM_QWEN_TIMEOUT:-5m}"
maxTokens = ${ENV_GONOTELM_QWEN_MAX_TOKENS:-8192}
temperature = ${ENV_GONOTELM_QWEN_TEMPERATURE:-1.0}
topP = ${ENV_GONOTELM_QWEN_TOP_P:-1.0}
enableThinking = ${ENV_GONOTELM_QWEN_ENABLE_THINKING:-true}
