package llm

import (
	"fmt"

	pkgrerank "github.com/gonotelm-lab/gonotelm/pkg/rerank"
	"github.com/gonotelm-lab/gonotelm/pkg/rerank/dashscope"
)

func newReranker(
	provider RerankProvider,
	cfg *RerankConfig,
	opts ...pkgrerank.ClientOption,
) (pkgrerank.Reranker, error) {
	if cfg == nil {
		return nil, fmt.Errorf("rerank config must not be nil")
	}

	switch provider {
	case RerankDashScope:
		return dashscope.New(cfg.DashScope, opts...)
	default:
		return nil, fmt.Errorf("rerank provider %q is not supported", provider)
	}
}
