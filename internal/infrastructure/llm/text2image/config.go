package text2image

import (
	"github.com/gonotelm-lab/multimodal/image/agnes"
	"github.com/gonotelm-lab/multimodal/image/dashscope"
)

type Text2ImageProvider string

func (t Text2ImageProvider) String() string {
	return string(t)
}

const (
	Text2ImageQwen  Text2ImageProvider = "qwen"
	Text2ImageAgnes Text2ImageProvider = "agnes"
)

type Text2ImageConfig struct {
	Qwen  dashscope.Config `toml:"qwen"`
	Agnes agnes.Config     `toml:"agnes"`
}
