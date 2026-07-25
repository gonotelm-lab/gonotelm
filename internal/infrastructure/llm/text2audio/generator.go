package text2audio

import (
	"fmt"

	audios "github.com/gonotelm-lab/multimodal/audio"
	"github.com/gonotelm-lab/multimodal/audio/dashscope"
	"github.com/gonotelm-lab/multimodal/audio/minimax"
	"github.com/gonotelm-lab/multimodal/audio/mimo"
)

func newText2AudioGenerator(
	provider Text2AudioProvider,
	cfg *Text2AudioConfig,
	opts ...audios.ClientOption,
) (audios.Generator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("text2audio config must not be nil")
	}

	switch provider {
	case Text2AudioDashScope:
		return dashscope.New(cfg.DashScope, opts...)
	case Text2AudioMimo:
		return mimo.New(cfg.Mimo, opts...)
	case Text2AudioMiniMax:
		return minimax.New(cfg.MiniMax, opts...)
	default:
		return nil, fmt.Errorf("text2audio provider %q is not supported", provider)
	}
}