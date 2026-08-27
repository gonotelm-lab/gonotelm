package schema

import (
	"time"

	"github.com/shopspring/decimal"
)

type EmbeddingLog struct {
	ID                  string                     `ch:"id"`
	GroupID             string                     `ch:"group_id"`
	TraceID             string                     `ch:"trace_id"`
	UserID              string                     `ch:"user_id"`
	Scene               string                     `ch:"scene"`
	Model               string                     `ch:"model"`
	ModelProvider       string                     `ch:"model_provider"`
	ModelParameters     *string                    `ch:"model_parameters"`
	CallStartTime       time.Time                  `ch:"call_start_time"`
	CallFinishTime      time.Time                  `ch:"call_finish_time"`
	InputCount          uint32                     `ch:"input_count"`
	EmbeddingCount      uint32                     `ch:"embedding_count"`
	EmbeddingDimensions uint32                     `ch:"embedding_dimensions"`
	UsageDetails        map[string]uint64          `ch:"usage_details"`
	CostDetails         map[string]decimal.Decimal `ch:"cost_details"`
	TotalCost           *decimal.Decimal           `ch:"total_cost"`
	CreateTime          time.Time                  `ch:"create_time"`
	Metadata            map[string]string          `ch:"metadata"`
	Error               *string                    `ch:"error"`
}

func (EmbeddingLog) TableName() string {
	return "embedding_logs"
}
