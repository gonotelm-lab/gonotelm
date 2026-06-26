deployEnv = "dev"

[api]
port = 7099
exitWaitTimeout = "${GONOTELM_API_EXIT_WAIT_TIMEOUT:-30s}"

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

[msgQueue]
type = "kafka"

[msgQueue.kafka]
brokers = ["${GONOTELM_KAFKA_BROKER:-127.0.0.1:9094}"]
username = "${GONOTELM_KAFKA_USERNAME:-kafka}"
password = "${GONOTELM_KAFKA_PASSWORD:-kafka}"
consumerQueueCapacity = ${GONOTELM_KAFKA_CONSUMER_QUEUE_CAPACITY:-100}
consumerCommitInterval = "${GONOTELM_KAFKA_CONSUMER_COMMIT_INTERVAL:-0s}"

[logging]
level = "${GONOTELM_LOG_LEVEL:-debug}"

[logic]

[logic.chat]
modelProvider = "${GONOTELM_LOGIC_CHAT_PROVIDER:-qwen}"
model = "${GONOTELM_LOGIC_CHAT_MODEL:-glm-5.1}"
maxRound = ${GONOTELM_LOGIC_CHAT_MAX_ROUND:-10}
rerankEnabled = ${GONOTELM_LOGIC_CHAT_RERANK_ENABLED:-false}
rerankProvider = "${GONOTELM_LOGIC_RERANK_PROVIDER:-dashscope}"
rerankTopN = ${GONOTELM_LOGIC_CHAT_RERANK_TOP_N:-30}
rerankModel = "${GONOTELM_LOGIC_CHAT_RERANK_MODEL:-qwen3-rerank}"

[logic.source]
modelProvider = "${GONOTELM_LOGIC_SOURCE_PROVIDER:-qwen}"
model = "${GONOTELM_LOGIC_SOURCE_MODEL:-qwen3.5-27b}"

[logic.source.bizCache]
eviction = "${GONOTELM_LOGIC_SOURCE_BIZCACHE_EVICTION:-15m}"
maxMB = ${GONOTELM_LOGIC_SOURCE_BIZCACHE_MAX_MB:-1024}

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

[logic.studio.audioOverview]
maxRound = ${GONOTELM_LOGIC_STUDIO_AUDIOOVERVIEW_MAX_ROUND:-50}
modelProvider = "${GONOTELM_LOGIC_STUDIO_AUDIOOVERVIEW_PROVIDER:-deepseek}"
model = "${GONOTELM_LOGIC_STUDIO_AUDIOOVERVIEW_MODEL:-deepseek-v4-flash}"

[chunking]
size = ${GONOTELM_CHUNKING_SIZE:-500}
overlapSize = ${GONOTELM_CHUNKING_OVERLAP_SIZE:-75}

[embedding]
type = "${GONOTELM_EMBEDDING_TYPE:-dashscope}"
batchSize = ${GONOTELM_EMBEDDING_BATCH_SIZE:-10}
maxConcurrency = ${GONOTELM_EMBEDDING_MAX_CONCURRENCY:-4}

[embedding.ark]
apiKey = "${GONOTELM_ARK_API_KEY:-}"
accessKey = "${GONOTELM_ARK_ACCESS_KEY:-}"
secretKey = "${GONOTELM_ARK_SECRET_KEY:-}"
baseUrl = "${GONOTELM_ARK_BASE_URL:-https://ark.cn-beijing.volces.com/api/v3}"
region = "${GONOTELM_ARK_REGION:-cn-beijing}"
model = "${GONOTELM_ARK_MODEL:-}"
apiType = "${GONOTELM_ARK_API_TYPE:-text_api}"
timeout = "${GONOTELM_ARK_TIMEOUT:-10m}"
retryTimes = ${GONOTELM_ARK_RETRY_TIMES:-2}
maxConcurrentRequests = ${GONOTELM_ARK_MAX_CONCURRENT_REQUESTS:-5}

[embedding.dashscope]
apiKey = "${GONOTELM_DASHSCOPE_API_KEY:-}"
model = "${GONOTELM_DASHSCOPE_MODEL:-text-embedding-v4}"
timeout = "${GONOTELM_DASHSCOPE_TIMEOUT:-30s}"
dimensions = ${GONOTELM_DASHSCOPE_DIMENSIONS:-1024}

[embedding.gemini]
apiKey = "${GONOTELM_GEMINI_API_KEY:-}"
backend = "${GONOTELM_GEMINI_BACKEND:-gemini_api}"
project = "${GONOTELM_GEMINI_PROJECT:-}"
location = "${GONOTELM_GEMINI_LOCATION:-}"
model = "${GONOTELM_GEMINI_MODEL:-gemini-embedding-001}"
taskType = "${GONOTELM_GEMINI_TASK_TYPE:-RETRIEVAL_DOCUMENT}"
title = "${GONOTELM_GEMINI_TITLE:-}"
outputDimensionality = ${GONOTELM_GEMINI_EMBED_DIMENSIONS:-1024}
mimeType = "${GONOTELM_GEMINI_MIME_TYPE:-text/plain}"
autoTruncate = ${GONOTELM_GEMINI_AUTO_TRUNCATE:-true}

[embedding.ollama]
baseUrl = "${GONOTELM_OLLAMA_BASE_URL:-http://localhost:11434}"
model = "${GONOTELM_OLLAMA_MODEL:-bge-m3}"
timeout = "${GONOTELM_OLLAMA_TIMEOUT:-30s}"
truncate = ${GONOTELM_OLLAMA_TRUNCATE:-false}
keepAlive = "${GONOTELM_OLLAMA_KEEP_ALIVE:-5m}"

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

[embedding.qianfan]
ak = "${GONOTELM_QIANFAN_AK:-}"
sk = "${GONOTELM_QIANFAN_SK:-}"
accessKey = "${GONOTELM_QIANFAN_ACCESS_KEY:-}"
secretKey = "${GONOTELM_QIANFAN_SECRET_KEY:-}"
accessToken = "${GONOTELM_QIANFAN_ACCESS_TOKEN:-}"
bearerToken = "${GONOTELM_QIANFAN_BEARER_TOKEN:-}"
model = "${GONOTELM_QIANFAN_MODEL:-Embedding-V1}"

[embedding.tencentcloud]
secretId = "${GONOTELM_TENCENTCLOUD_SECRET_ID:-}"
secretKey = "${GONOTELM_TENCENTCLOUD_SECRET_KEY:-}"
region = "${GONOTELM_TENCENTCLOUD_REGION:-ap-guangzhou}"

[rerank]
type = "${GONOTELM_RERANK_TYPE:-dashscope}"

[rerank.dashscope]
apiKey = "${GONOTELM_DASHSCOPE_API_KEY:-}"
baseUrl = "${GONOTELM_RERANK_DASHSCOPE_BASE_URL:-https://dashscope.aliyuncs.com/compatible-api/v1/reranks}"
model = "${GONOTELM_RERANK_DASHSCOPE_MODEL:-qwen3-rerank}"
timeout = "${GONOTELM_RERANK_DASHSCOPE_TIMEOUT:-30s}"

[provider]

[provider.deepseek]
apiKey = "${GONOTELM_DEEPSEEK_API_KEY}"
timeout = "${GONOTELM_DEEPSEEK_TIMEOUT:-5m}"
baseUrl = "${GONOTELM_DEEPSEEK_BASE_URL:-https://api.deepseek.com}"
model = "${GONOTELM_DEEPSEEK_MODEL:-deepseek-v4-flash}"
maxTokens = ${GONOTELM_DEEPSEEK_MAX_TOKENS:-16384}
thinkingEnabled = false

[provider.openai]
apiKey = "${GONOTELM_OPENAI_API_KEY:-}"
baseUrl = "${GONOTELM_OPENAI_BASE_URL:-https://api.openai.com/v1}"
model = "${GONOTELM_OPENAI_MODEL:-gpt-4o-mini}"
timeout = "${GONOTELM_OPENAI_TIMEOUT:-5m}"
maxTokens = ${GONOTELM_OPENAI_MAX_TOKENS:-16384}
temperature = ${GONOTELM_OPENAI_TEMPERATURE:-1.0}
reasoningEffort = "${GONOTELM_OPENAI_REASONING_EFFORT:-}"

[provider.qwen]
apiKey = "${GONOTELM_DASHSCOPE_API_KEY:-}"
baseUrl = "${GONOTELM_QWEN_BASE_URL:-https://dashscope.aliyuncs.com/compatible-mode/v1}"
model = "${GONOTELM_QWEN_MODEL:-glm-5.1}"
timeout = "${GONOTELM_QWEN_TIMEOUT:-5m}"
maxTokens = ${GONOTELM_QWEN_MAX_TOKENS:-16384}
temperature = ${GONOTELM_QWEN_TEMPERATURE:-1.0}
topP = ${GONOTELM_QWEN_TOP_P:-1.0}
enableThinking = ${GONOTELM_QWEN_ENABLE_THINKING:-false}

[provider.agnes]
apiKey = "${GONOTELM_AGNES_API_KEY:-}"
baseUrl = "${GONOTELM_AGNES_BASE_URL:-https://apihub.agnes-ai.com/v1}"
model = "${GONOTELM_AGNES_MODEL:-agnes-2.0-flash}"
timeout = "${GONOTELM_AGNES_TIMEOUT:-5m}"
maxTokens = ${GONOTELM_AGNES_MAX_TOKENS:-16384}
temperature = ${GONOTELM_AGNES_TEMPERATURE:-1.0}
topP = ${GONOTELM_AGNES_TOP_P:-1.0}

[text2image]
type = "${GONOTELM_TEXT2IMAGE_TYPE:-dashscope}"

[text2image.dashscope]
apiKey = "${GONOTELM_DASHSCOPE_API_KEY:-}"
baseUrl = "${GONOTELM_TEXT2IMAGE_DASHSCOPE_BASE_URL:-https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation}"
model = "${GONOTELM_TEXT2IMAGE_DASHSCOPE_MODEL:-qwen-image-2.0-pro}"
timeout = "${GONOTELM_TEXT2IMAGE_DASHSCOPE_TIMEOUT:-1h}"

[text2image.agnes]
apiKey = "${GONOTELM_AGNES_API_KEY:-}"
baseUrl = "${GONOTELM_TEXT2IMAGE_AGNES_BASE_URL:-https://apihub.agnes-ai.com/v1/images/generations}"
model = "${GONOTELM_TEXT2IMAGE_AGNES_MODEL:-agnes-image-2.1-flash}"
timeout = "${GONOTELM_TEXT2IMAGE_AGNES_TIMEOUT:-1h}"
