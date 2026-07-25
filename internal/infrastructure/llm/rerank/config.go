package rerank

import "github.com/gonotelm-lab/gonotelm/pkg/rerank/dashscope"

type RerankProvider string

func (t RerankProvider) String() string {
	return string(t)
}

const (
	RerankDashScope RerankProvider = "dashscope"
)

type RerankConfig struct {
	DashScope dashscope.Config `toml:"dashscope"`
}
