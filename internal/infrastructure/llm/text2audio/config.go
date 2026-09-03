package text2audio

import (
	"github.com/gonotelm-lab/multimodal/audio/dashscope"
	"github.com/gonotelm-lab/multimodal/audio/mimo"
	"github.com/gonotelm-lab/multimodal/audio/minimax"
)

type Text2AudioProvider string

func (t Text2AudioProvider) String() string {
	return string(t)
}

const (
	Text2AudioQwen    Text2AudioProvider = "qwen"
	Text2AudioMimo    Text2AudioProvider = "mimo"
	Text2AudioMiniMax Text2AudioProvider = "minimax"
)

type Text2AudioConfig struct {
	Qwen    dashscope.Config `toml:"qwen"`
	Mimo    mimo.Config      `toml:"mimo"`
	MiniMax minimax.Config   `toml:"minimax"`
}
