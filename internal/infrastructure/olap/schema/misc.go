package schema

import "time"

type TimeRange struct {
	Start time.Time
	End   time.Time
}

type LimitOffset struct {
	Offset int
	Limit  int
}

// ExtraQueryConditions is shared by all OLAP log Query APIs.
type ExtraQueryConditions struct {
	LimitOffset

	GroupId string
	TraceId string
}
