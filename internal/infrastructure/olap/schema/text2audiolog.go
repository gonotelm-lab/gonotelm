package schema

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/clickhouse/util"
	"github.com/shopspring/decimal"
)

type Text2AudioLog struct {
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
	Text            *string                    `ch:"text"`
	UsageDetails    map[string]uint64          `ch:"usage_details"`
	CostDetails     map[string]decimal.Decimal `ch:"cost_details"`
	TotalCost       *decimal.Decimal           `ch:"total_cost"`
	CreateTime      time.Time                  `ch:"create_time"`
	Metadata        map[string]string          `ch:"metadata"`
	Error           *string                    `ch:"error"`
}

var Text2AudioLogAllFields = util.GetFields(&Text2AudioLog{})

const (
	Text2AudioLogTableName = "text2audio_logs"
	Text2AudioLogSchema    = `
CREATE TABLE IF NOT EXISTS text2audio_logs (
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
	text Nullable(String),
	usage_details Map(LowCardinality(String), UInt64),
	cost_details Map(LowCardinality(String), Decimal64(12)),
	total_cost Nullable(Decimal64(12)),
	create_time DateTime64(3),
	metadata Map(LowCardinality(String), String),
	error Nullable(String)
) ENGINE = Memory`
)

func (Text2AudioLog) TableName() string {
	return Text2AudioLogTableName
}
