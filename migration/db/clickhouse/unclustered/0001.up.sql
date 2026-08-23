CREATE DATABASE IF NOT EXISTS gonotelm;

CREATE TABLE IF NOT EXISTS gonotelm.llm_logs (
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
	`update_time` DateTime64(3) DEFAULT now64(3),
	`metadata` Map(LowCardinality(String), String),
	`error` Nullable(String),
	`is_deleted` UInt8
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(create_time)
ORDER BY (group_id, trace_id, create_time)
PRIMARY KEY (group_id, trace_id);