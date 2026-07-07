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
	Type      RerankProvider    `toml:"type"`
	DashScope dashscope.Config `toml:"dashscope"`
}
