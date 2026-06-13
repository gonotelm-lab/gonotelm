package text2image

import (
	"github.com/gonotelm-lab/gonotelm/pkg/text2image/dashscope"
	"github.com/gonotelm-lab/gonotelm/pkg/text2image/agnes"
)

type Provider string

func (t Provider) String() string {
	return string(t)
}

const (
	DashScope Provider = "dashscope"
	Agnes     Provider = "agnes"
)

type Config struct {
	Type      Provider         `toml:"type"`
	DashScope dashscope.Config `toml:"dashscope"`
	Agnes     agnes.Config     `toml:"agnes"`
}
