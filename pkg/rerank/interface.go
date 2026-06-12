package rerank

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/pkg/rerank/schema"
)

type Reranker interface {
	Rerank(ctx context.Context, req schema.Request, opts ...Option) (schema.Response, error)
}
