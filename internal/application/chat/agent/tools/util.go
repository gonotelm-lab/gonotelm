package tools

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

const OkToolCallResult = "OK"

// 检查LLM是否允许访问特定的Sources
type SourcePermissionChecker interface {
	Check(ctx context.Context, sourceIds []valobj.Id) error
}

type SourcePermissionCheckerFunc func(ctx context.Context, sourceIds []valobj.Id) error

func (f SourcePermissionCheckerFunc) Check(ctx context.Context, sourceIds []valobj.Id) error {
	return f(ctx, sourceIds)
}

// 收集 LLM 在回答中引用的 SourceDoc
type CitationCollector interface {
	Set(sourceDocIds []valobj.Id)
}

type CitationCollectorFunc func(sourceDocIds []valobj.Id)

func (f CitationCollectorFunc) Set(sourceDocIds []valobj.Id) {
	f(sourceDocIds)
}

type SourceDocPermissionChecker interface {
	Check(ctx context.Context, sourceDocIds []valobj.Id) error
}

type SourceDocPermissionCheckerFunc func(ctx context.Context, sourceDocIds []valobj.Id) error

func (f SourceDocPermissionCheckerFunc) Check(ctx context.Context, sourceDocIds []valobj.Id) error {
	return f(ctx, sourceDocIds)
}
