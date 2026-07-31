package text2audio

import (
	"fmt"
	"time"

	audios "github.com/gonotelm-lab/multimodal/audio"
	"github.com/gonotelm-lab/multimodal/audio/dashscope"
	"github.com/gonotelm-lab/multimodal/audio/minimax"
	"github.com/gonotelm-lab/multimodal/audio/mimo"

	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
)

const defaultGenerateTimeout = 30 * time.Minute

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
		return dashscope.New(cfg.DashScope, withAudioHTTPClient(opts, cfg.DashScope.Timeout)...)
	case Text2AudioMimo:
		return mimo.New(cfg.Mimo, withAudioHTTPClient(opts, cfg.Mimo.Timeout)...)
	case Text2AudioMiniMax:
		return minimax.New(cfg.MiniMax, withAudioHTTPClient(opts, cfg.MiniMax.Timeout)...)
	default:
		return nil, fmt.Errorf("text2audio provider %q is not supported", provider)
	}
}

func withAudioHTTPClient(opts []audios.ClientOption, timeout time.Duration) []audios.ClientOption {
	if timeout <= 0 {
		timeout = defaultGenerateTimeout
	}
	client := httpclient.NewBuilder(nil).WithTimeout(timeout).Build()
	return append(append([]audios.ClientOption(nil), opts...), audios.WithHTTPClient(client))
}