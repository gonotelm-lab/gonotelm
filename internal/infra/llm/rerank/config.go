package rerank

import "github.com/gonotelm-lab/gonotelm/pkg/rerank/dashscope"

type Provider string

func (t Provider) String() string {
	return string(t)
}

const (
	DashScope Provider = "dashscope"
)

type Config struct {
	Type      Provider         `toml:"type"`
	DashScope dashscope.Config `toml:"dashscope"`
}
