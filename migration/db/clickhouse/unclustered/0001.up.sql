CREATE DATABASE IF NOT EXISTS gonotelm;

CREATE TABLE IF NOT EXISTS llm_logs (
	`id` String,
	`group_id` String,
	`trace_id` String,
	`user_id` String,
	`scene` LowCardinality(String),
	`model` LowCardinality(String),
	`model_provider` LowCardinality(String),
	`model_parameters` Nullable(String),
	`call_start_time` DateTime64(3),
	`call_finish_time` DateTime64(3),
	`input` Nullable(String) CODEC(ZSTD(3)),
	`output` Nullable(String) CODEC(ZSTD(3)),
	`tool_definitions` Map(LowCardinality(String), String),
	`tool_calls` Array(Tuple(name LowCardinality(String), arguments String)),
	`usage_details` Map(LowCardinality(String), UInt64),
	`cost_details` Map(LowCardinality(String), Decimal64(12)),
	`total_cost` Nullable(Decimal64(12)),
	`create_time` DateTime64(3) DEFAULT now64(3),
	`metadata` Map(LowCardinality(String), String),
	`error` Nullable(String),

	INDEX idx_id id TYPE bloom_filter(0.01) GRANULARITY 4,
	INDEX idx_group_id group_id TYPE bloom_filter(0.01) GRANULARITY 4,
	INDEX idx_trace_id trace_id TYPE bloom_filter(0.01) GRANULARITY 4,
	INDEX idx_scene scene TYPE set(0) GRANULARITY 4,
	INDEX idx_model model TYPE set(0) GRANULARITY 4
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(call_start_time)
ORDER BY (user_id, call_start_time, id)
PRIMARY KEY (user_id, call_start_time)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS embedding_logs (
	`id` String,
	`group_id` String,
	`trace_id` String,
	`user_id` String,
	`scene` LowCardinality(String),
	`model` LowCardinality(String),
	`model_provider` LowCardinality(String),
	`model_parameters` Nullable(String),
	`call_start_time` DateTime64(3),
	`call_finish_time` DateTime64(3),
	`input_count` UInt32,
	`embedding_count` UInt32,
	`embedding_dimensions` UInt32,
	`usage_details` Map(LowCardinality(String), UInt64),
	`cost_details` Map(LowCardinality(String), Decimal64(12)),
	`total_cost` Nullable(Decimal64(12)),
	`create_time` DateTime64(3) DEFAULT now64(3),
	`metadata` Map(LowCardinality(String), String),
	`error` Nullable(String),

	INDEX idx_id id TYPE bloom_filter(0.01) GRANULARITY 4,
	INDEX idx_group_id group_id TYPE bloom_filter(0.01) GRANULARITY 4,
	INDEX idx_trace_id trace_id TYPE bloom_filter(0.01) GRANULARITY 4,
	INDEX idx_scene scene TYPE set(0) GRANULARITY 4,
	INDEX idx_model model TYPE set(0) GRANULARITY 4
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(call_start_time)
ORDER BY (user_id, call_start_time, id)
PRIMARY KEY (user_id, call_start_time)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS text2image_logs (
	`id` String,
	`group_id` String,
	`trace_id` String,
	`user_id` String,
	`scene` LowCardinality(String),
	`model` LowCardinality(String),
	`model_provider` LowCardinality(String),
	`model_parameters` Nullable(String),
	`call_start_time` DateTime64(3),
	`call_finish_time` DateTime64(3),
	`prompt` Nullable(String) CODEC(ZSTD(3)),
	`usage_details` Map(LowCardinality(String), UInt64),
	`cost_details` Map(LowCardinality(String), Decimal64(12)),
	`total_cost` Nullable(Decimal64(12)),
	`create_time` DateTime64(3) DEFAULT now64(3),
	`metadata` Map(LowCardinality(String), String),
	`error` Nullable(String),

	INDEX idx_id id TYPE bloom_filter(0.01) GRANULARITY 4,
	INDEX idx_group_id group_id TYPE bloom_filter(0.01) GRANULARITY 4,
	INDEX idx_trace_id trace_id TYPE bloom_filter(0.01) GRANULARITY 4,
	INDEX idx_scene scene TYPE set(0) GRANULARITY 4,
	INDEX idx_model model TYPE set(0) GRANULARITY 4
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(call_start_time)
ORDER BY (user_id, call_start_time, id)
PRIMARY KEY (user_id, call_start_time)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS text2audio_logs (
	`id` String,
	`group_id` String,
	`trace_id` String,
	`user_id` String,
	`scene` LowCardinality(String),
	`model` LowCardinality(String),
	`model_provider` LowCardinality(String),
	`model_parameters` Nullable(String),
	`call_start_time` DateTime64(3),
	`call_finish_time` DateTime64(3),
	`text` Nullable(String) CODEC(ZSTD(3)),
	`usage_details` Map(LowCardinality(String), UInt64),
	`cost_details` Map(LowCardinality(String), Decimal64(12)),
	`total_cost` Nullable(Decimal64(12)),
	`create_time` DateTime64(3) DEFAULT now64(3),
	`metadata` Map(LowCardinality(String), String),
	`error` Nullable(String),

	INDEX idx_id id TYPE bloom_filter(0.01) GRANULARITY 4,
	INDEX idx_group_id group_id TYPE bloom_filter(0.01) GRANULARITY 4,
	INDEX idx_trace_id trace_id TYPE bloom_filter(0.01) GRANULARITY 4,
	INDEX idx_scene scene TYPE set(0) GRANULARITY 4,
	INDEX idx_model model TYPE set(0) GRANULARITY 4
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(call_start_time)
ORDER BY (user_id, call_start_time, id)
PRIMARY KEY (user_id, call_start_time)
SETTINGS index_granularity = 8192;
