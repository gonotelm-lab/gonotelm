package text2image

import (
	"github.com/gonotelm-lab/multimodal/image/dashscope"
	"github.com/gonotelm-lab/multimodal/image/agnes"
)

type Text2ImageProvider string

func (t Text2ImageProvider) String() string {
	return string(t)
}

const (
	Text2ImageDashScope Text2ImageProvider = "dashscope"
	Text2ImageAgnes     Text2ImageProvider = "agnes"
)

type Text2ImageConfig struct {
	Type      Text2ImageProvider    `toml:"type"`
	DashScope dashscope.Config `toml:"dashscope"`
	Agnes     agnes.Config     `toml:"agnes"`
}
