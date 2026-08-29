package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/shopspring/decimal"
)

const scriptedCharacterKey = "character_10k"

type characterPriceProviderUsageEnv struct {
	Characters int64 `expr:"characters"`
}

type characterPriceProviderEnv struct {
	Model string                         `expr:"model"`
	Now   time.Time                      `expr:"now"`
	Usage characterPriceProviderUsageEnv `expr:"usage"`
}

type ScriptedCharacterPriceProvider struct {
	vm  *vm.Program
	now func() time.Time
}

func NewScriptedCharacterPriceProvider(script string) (*ScriptedCharacterPriceProvider, error) {
	program, err := expr.Compile(script, expr.Env(characterPriceProviderEnv{}))
	if err != nil {
		return nil, fmt.Errorf("compile scripted character price provider script err: %w", err)
	}

	return &ScriptedCharacterPriceProvider{
		vm:  program,
		now: time.Now,
	}, nil
}

func (p *ScriptedCharacterPriceProvider) Provide(
	ctx context.Context,
	model string,
	usage text2audio.RecordUsage,
) (CharacterPrices, error) {
	now := time.Now()
	if p.now != nil {
		now = p.now()
	}

	var characters int64
	if usage.Characters != nil {
		characters = *usage.Characters
	}

	runOut, err := expr.Run(p.vm, characterPriceProviderEnv{
		Model: model,
		Now:   now,
		Usage: characterPriceProviderUsageEnv{Characters: characters},
	})
	if err != nil {
		return CharacterPrices{}, fmt.Errorf("scripted character price provider run err: %w", err)
	}

	if errMsg, ok := runOut.(string); ok {
		return CharacterPrices{}, fmt.Errorf("scripted character price provider error: %s", errMsg)
	}

	prices, ok := runOut.(map[string]any)
	if !ok {
		return CharacterPrices{}, fmt.Errorf("scripted character provider output is not map[string]any but %T", runOut)
	}

	characterRaw, ok := prices[scriptedCharacterKey].(string)
	if !ok {
		return CharacterPrices{}, ErrMissingCharacterPrice
	}

	characterPrice, err := decimal.NewFromString(characterRaw)
	if err != nil {
		return CharacterPrices{}, ErrCharacterNotNumber
	}

	return CharacterPrices{CharacterPrice: characterPrice}, nil
}
