package billing

import (
	"context"
	"fmt"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	"github.com/shopspring/decimal"
)

// StandardMeter routes providers to a concrete billing meter.
//
// Current routing:
//   - qwen / minimax → character-based
//   - mimo → token-based?
type StandardMeter struct {
	mu     sync.RWMutex
	routes map[text2audio.Text2AudioProvider]Meter
}

type StandardMeterConfig struct {
	// Character-based provider scripts (key: character_10k)
	QwenScript    string
	MiniMaxScript string
}

func NewStandardMeter(c StandardMeterConfig) (Meter, error) {
	characterMeter := NewCharacterBasedMeter()
	routes := make(map[text2audio.Text2AudioProvider]Meter, 4)

	if len(c.QwenScript) > 0 {
		qwen, err := NewScriptedCharacterPriceProvider(c.QwenScript)
		if err != nil {
			return nil, fmt.Errorf("init qwen character script err: %w", err)
		}
		characterMeter.SetProvider(text2audio.Text2AudioQwen, qwen)
		routes[text2audio.Text2AudioQwen] = characterMeter
	}
	if len(c.MiniMaxScript) > 0 {
		miniMax, err := NewScriptedCharacterPriceProvider(c.MiniMaxScript)
		if err != nil {
			return nil, fmt.Errorf("init minimax character script err: %w", err)
		}
		characterMeter.SetProvider(text2audio.Text2AudioMiniMax, miniMax)
		routes[text2audio.Text2AudioMiniMax] = characterMeter
	}

	return &StandardMeter{routes: routes}, nil
}

var _ Meter = &StandardMeter{}

func (m *StandardMeter) Calculate(
	ctx context.Context,
	provider text2audio.Text2AudioProvider,
	model string,
	usage text2audio.RecordUsage,
) (*decimal.Decimal, map[PriceDetailKey]decimal.Decimal, error) {
	m.mu.RLock()
	meter, ok := m.routes[provider]
	m.mu.RUnlock()
	if !ok || meter == nil {
		// not billed
		return nil, nil, nil
	}
	return meter.Calculate(ctx, provider, model, usage)
}
