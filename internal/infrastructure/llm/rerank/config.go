package rerank

import "github.com/gonotelm-lab/gonotelm/pkg/rerank/qwen"

type RerankProvider string

func (t RerankProvider) String() string {
	return string(t)
}

const (
	RerankQwen RerankProvider = "qwen"
)

type RerankConfig struct {
	Qwen qwen.Config `toml:"qwen"`
}
