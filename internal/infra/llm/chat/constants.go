package chat

import "github.com/gonotelm-lab/gonotelm/pkg/llm"

const (
	FinishReasonStop          = llm.FinishReasonStop
	FinishReasonLength        = llm.FinishReasonLength
	FinishReasonToolCalls     = llm.FinishReasonToolCalls
	FinishReasonContentFilter = llm.FinishReasonContentFilter
)
