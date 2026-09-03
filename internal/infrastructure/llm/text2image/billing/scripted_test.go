package billing

import (
	"context"
	_ "embed"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/qwen.text2image
	testQwenText2ImageScript string
)

func TestNewScriptedPriceProvider_CompilesModelPriceScripts(t *testing.T) {
	_, err := NewScriptedPriceProvider(testQwenText2ImageScript)
	require.NoError(t, err)
}

func TestNewScriptedPriceProvider_InvalidScript(t *testing.T) {
	_, err := NewScriptedPriceProvider("{{{")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compile")
}

func TestScriptedPriceProvider_QwenText2Image(t *testing.T) {
	p, err := NewScriptedPriceProvider(testQwenText2ImageScript)
	require.NoError(t, err)

	cases := []struct {
		name  string
		model string
		want  string
	}{
		{"qwen-image-3.0", "qwen-image-3.0", "0.18"},
		{"qwen-image-3.0-pro", "qwen-image-3.0-pro", "0.25"},
		{"qwen-image-2.0-pro", "qwen-image-2.0-pro", "0.5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.Provide(context.Background(), tc.model, text2image.RecordUsage{OutputCount: 1})
			require.NoError(t, err)
			want := decimal.RequireFromString(tc.want)
			assert.True(t, want.Equal(got.ImagePrice), "image got=%s want=%s", got.ImagePrice, want)
		})
	}
}

func TestStandardMeter_CalculateByOutputCount(t *testing.T) {
	meter, err := NewStandardMeter(StandardMeterConfig{QwenScript: testQwenText2ImageScript})
	require.NoError(t, err)

	total, details, err := meter.Calculate(
		context.Background(),
		text2image.Text2ImageQwen,
		"qwen-image-3.0",
		text2image.RecordUsage{OutputCount: 2},
	)
	require.NoError(t, err)
	require.NotNil(t, total)
	assert.True(t, decimal.RequireFromString("0.36").Equal(*total))
	assert.True(t, decimal.RequireFromString("0.36").Equal(details[ImagePriceKey]))
}

func TestScriptedPriceProvider_UnknownModel(t *testing.T) {
	p, err := NewScriptedPriceProvider(testQwenText2ImageScript)
	require.NoError(t, err)

	_, err = p.Provide(context.Background(), "unknown-model", text2image.RecordUsage{OutputCount: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model not supported to provide price")
}

func TestScriptedPriceProvider_MissingImageKey(t *testing.T) {
	p, err := NewScriptedPriceProvider(`{prompt_1m: "1"}`)
	require.NoError(t, err)

	_, err = p.Provide(context.Background(), "any", text2image.RecordUsage{})
	require.ErrorIs(t, err, ErrMissingImagePrice)
}

func TestScriptedPriceProvider_InvalidPriceNumber(t *testing.T) {
	p, err := NewScriptedPriceProvider(`{image: "abc"}`)
	require.NoError(t, err)

	_, err = p.Provide(context.Background(), "any", text2image.RecordUsage{})
	require.ErrorIs(t, err, ErrImageNotNumber)
}
