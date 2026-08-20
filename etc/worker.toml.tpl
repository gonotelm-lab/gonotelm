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

[studio.mindmap]
maxRound = ${GONOTELM_STUDIO_MINDMAP_MAX_ROUND:-50}
modelProvider = "${GONOTELM_STUDIO_MINDMAP_PROVIDER:-deepseek}"
model = "${GONOTELM_STUDIO_MINDMAP_MODEL:-deepseek-v4-flash}"

[studio.report]
maxRound = ${GONOTELM_STUDIO_REPORT_MAX_ROUND:-50}
modelProvider = "${GONOTELM_STUDIO_REPORT_PROVIDER:-deepseek}"
model = "${GONOTELM_STUDIO_REPORT_MODEL:-deepseek-v4-flash}"

[studio.infoGraphic]
maxRound = ${GONOTELM_STUDIO_INFOGRAPHIC_MAX_ROUND:-50}
modelProvider = "${GONOTELM_STUDIO_INFOGRAPHIC_PROVIDER:-deepseek}"
model = "${GONOTELM_STUDIO_INFOGRAPHIC_MODEL:-deepseek-v4-flash}"
imageModelProvider = "${GONOTELM_STUDIO_INFOGRAPHIC_IMAGE_MODEL_PROVIDER:-dashscope}"
imageModel = "${GONOTELM_STUDIO_INFOGRAPHIC_IMAGE_MODEL:-qwen-image-2.0-pro}"

[studio.audioOverview]
maxRound = ${GONOTELM_STUDIO_AUDIOOVERVIEW_MAX_ROUND:-50}
modelProvider = "${GONOTELM_STUDIO_AUDIOOVERVIEW_PROVIDER:-deepseek}"
model = "${GONOTELM_STUDIO_AUDIOOVERVIEW_MODEL:-deepseek-v4-flash}"
audioModelProvider = "${GONOTELM_STUDIO_AUDIOOVERVIEW_AUDIO_MODEL_PROVIDER:-dashscope}"
audioModel = "${GONOTELM_STUDIO_AUDIOOVERVIEW_AUDIO_MODEL:-qwen3-tts-instruct-flash}"
audioSynthConcurrency = ${GONOTELM_STUDIO_AUDIOOVERVIEW_AUDIO_SYNTH_CONCURRENCY:-1}

[studio.flashcard]
maxRound = ${GONOTELM_STUDIO_FLASHCARD_MAX_ROUND:-50}
modelProvider = "${GONOTELM_STUDIO_FLASHCARD_PROVIDER:-deepseek}"
model = "${GONOTELM_STUDIO_FLASHCARD_MODEL:-deepseek-v4-flash}"

[studio.quiz]
maxRound = ${GONOTELM_STUDIO_QUIZ_MAX_ROUND:-50}
modelProvider = "${GONOTELM_STUDIO_QUIZ_PROVIDER:-deepseek}"
model = "${GONOTELM_STUDIO_QUIZ_MODEL:-deepseek-v4-flash}"

[studio.dataTable]
maxRound = ${GONOTELM_STUDIO_DATATABLE_MAX_ROUND:-50}
modelProvider = "${GONOTELM_STUDIO_DATATABLE_PROVIDER:-deepseek}"
model = "${GONOTELM_STUDIO_DATATABLE_MODEL:-deepseek-v4-flash}"

[studio.slides]
maxRound = ${GONOTELM_STUDIO_SLIDES_MAX_ROUND:-50}
modelProvider = "${GONOTELM_STUDIO_SLIDES_PROVIDER:-deepseek}"
generateMaxRound = ${GONOTELM_STUDIO_SLIDES_GENERATE_MAX_ROUND:-100}
model = "${GONOTELM_STUDIO_SLIDES_MODEL:-deepseek-v4-flash}"
sandboxProvider = "${GONOTELM_STUDIO_SLIDES_SANDBOX_PROVIDER:-opensandbox}"

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
thinkingEnabled = false

[provider.qwen]
apiKey = "${GONOTELM_DASHSCOPE_API_KEY:-}"
baseUrl = "${GONOTELM_QWEN_BASE_URL:-https://dashscope.aliyuncs.com/compatible-mode/v1}"
model = "${GONOTELM_QWEN_MODEL:-glm-5.1}"
timeout = "${GONOTELM_QWEN_TIMEOUT:-5m}"
temperature = ${GONOTELM_QWEN_TEMPERATURE:-1.0}
topP = ${GONOTELM_QWEN_TOP_P:-1.0}
enableThinking = ${GONOTELM_QWEN_ENABLE_THINKING:-false}

[provider.agnes]
apiKey = "${GONOTELM_AGNES_API_KEY:-}"
baseUrl = "${GONOTELM_AGNES_BASE_URL:-https://apihub.agnes-ai.com/v1}"
model = "${GONOTELM_AGNES_MODEL:-agnes-2.5-pro}"
timeout = "${GONOTELM_AGNES_TIMEOUT:-5m}"
temperature = ${GONOTELM_AGNES_TEMPERATURE:-1.0}
topP = ${GONOTELM_AGNES_TOP_P:-1.0}

[text2image.dashscope]
apiKey = "${GONOTELM_DASHSCOPE_API_KEY:-}"
baseUrl = "${GONOTELM_TEXT2IMAGE_DASHSCOPE_BASE_URL:-https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation}"
model = "${GONOTELM_TEXT2IMAGE_DASHSCOPE_MODEL:-qwen-image-2.0-pro}"
timeout = "${GONOTELM_TEXT2IMAGE_DASHSCOPE_TIMEOUT:-10m}"

[text2image.agnes]
apiKey = "${GONOTELM_AGNES_API_KEY:-}"
baseUrl = "${GONOTELM_TEXT2IMAGE_AGNES_BASE_URL:-https://apihub.agnes-ai.com/v1/images/generations}"
model = "${GONOTELM_TEXT2IMAGE_AGNES_MODEL:-agnes-image-2.1-flash}"
timeout = "${GONOTELM_TEXT2IMAGE_AGNES_TIMEOUT:-10m}"

[text2audio.dashscope]
apiKey = "${GONOTELM_DASHSCOPE_API_KEY:-}"
baseUrl = "${GONOTELM_TEXT2AUDIO_DASHSCOPE_BASE_URL:-https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation}"
model = "${GONOTELM_TEXT2AUDIO_DASHSCOPE_MODEL:-qwen3-tts-flash}"
timeout = "${GONOTELM_TEXT2AUDIO_DASHSCOPE_TIMEOUT:-10m}"

[text2audio.mimo]
apiKey = "${GONOTELM_MIMO_API_KEY:-}"
baseUrl = "${GONOTELM_TEXT2AUDIO_MIMO_BASE_URL:-https://api.xiaomimimo.com/v1/chat/completions}"
model = "${GONOTELM_TEXT2AUDIO_MIMO_MODEL:-}"
timeout = "${GONOTELM_TEXT2AUDIO_MIMO_TIMEOUT:-10m}"

[text2audio.minimax]
apiKey = "${GONOTELM_MINIMAX_API_KEY:-}"
baseUrl = "${GONOTELM_TEXT2AUDIO_MINIMAX_BASE_URL:-https://api.minimaxi.com/v1/t2a_v2}"
model = "${GONOTELM_TEXT2AUDIO_MINIMAX_MODEL:-}"
timeout = "${GONOTELM_TEXT2AUDIO_MINIMAX_TIMEOUT:-10m}"

[flow]
addr        = "${GONOTELM_FLOW_ADDR:-localhost:7091}"
namespace   = "${GONOTELM_FLOW_NAMESPACE:-gonotelm}"
maxRetry    = ${GONOTELM_FLOW_MAX_RETRY:-3}
dialTimeout = "${GONOTELM_FLOW_DIAL_TIMEOUT:-5s}"

[sandbox.opensandbox]
endpoint = "${GONOTELM_SANDBOX_ENDPOINT:-http://localhost:23080}"
apiKey = "${GONOTELM_SANDBOX_API_KEY:-123456}"
timeout = "${GONOTELM_SANDBOX_TIMEOUT:-30s}"
image = "${GONOTELM_SANDBOX_IMAGE:opensandbox/code-interpreter-base}"

[worker]
maxConcurrency  = ${GONOTELM_WORKER_MAX_CONCURRENCY:-4}
heartbeat       = "${GONOTELM_WORKER_HEARTBEAT:-5s}"

[otelTrace]
name = "worker"
endpoint = "${OTEL_TRACE_ENDPOINT:-127.0.0.1:4317}"
exporter = "grpc"
