package schema

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/clickhouse/util"
	"github.com/shopspring/decimal"
)

type LLMLog struct {
	Id              string                     `ch:"id"`
	GroupId         string                     `ch:"group_id"`
	TraceId         string                     `ch:"trace_id"`
	UserId          string                     `ch:"user_id"`
	Scene           string                     `ch:"scene"`
	Model           string                     `ch:"model"`
	ModelProvider   string                     `ch:"model_provider"`
	ModelParameters *string                    `ch:"model_parameters"`
	CallStartTime   time.Time                  `ch:"call_start_time"`
	CallFinishTime  time.Time                  `ch:"call_finish_time"`
	Input           *string                    `ch:"input"`
	Output          *string                    `ch:"output"`
	ToolDefinitions map[string]string          `ch:"tool_definitions"`
	ToolCalls       []LLMLogToolCall           `ch:"tool_calls"`
	UsageDetails    map[string]uint64          `ch:"usage_details"`
	CostDetails     map[string]decimal.Decimal `ch:"cost_details"`
	TotalCost       *decimal.Decimal           `ch:"total_cost"`
	CreateTime      time.Time                  `ch:"create_time"`
	Metadata        map[string]string          `ch:"metadata"`
	Error           *string                    `ch:"error"`
}

var LLMLogAllFields = util.GetFields(&LLMLog{})

const (
	LLMLogTableName = "llm_logs"
	LLMLogSchema    = `
CREATE TABLE IF NOT EXISTS llm_logs (
	id String,
	group_id String,
	trace_id String,
	user_id String,
	scene LowCardinality(String),
	model LowCardinality(String),
	model_provider LowCardinality(String),
	model_parameters Nullable(String),
	call_start_time DateTime64(3),
	call_finish_time DateTime64(3),
	input Nullable(String),
	output Nullable(String),
	tool_definitions Map(LowCardinality(String), String),
	tool_calls Array(Tuple(name LowCardinality(String), arguments String)),
	usage_details Map(LowCardinality(String), UInt64),
	cost_details Map(LowCardinality(String), Decimal64(12)),
	total_cost Nullable(Decimal64(12)),
	create_time DateTime64(3),
	metadata Map(LowCardinality(String), String),
	error Nullable(String)
) ENGINE = Memory`
)

type LLMLogToolCall struct {
	Name      string `ch:"name"`
	Arguments string `ch:"arguments"`
}

func (LLMLog) TableName() string {
	return LLMLogTableName
}
