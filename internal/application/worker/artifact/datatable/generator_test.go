package datatable

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	einoschema "github.com/cloudwego/eino/schema"
)

func TestRenderDataTable(t *testing.T) {
	msgs, err := RenderDataTable(t.Context(), []string{"src-1"}, "focus on metrics")
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Contains(t, msgs[0].Content, "src-1")
	assert.Equal(t, einoschema.User, msgs[1].Role)
	assert.Contains(t, msgs[1].Content, "focus on metrics")
}

func TestRenderTitleMaker(t *testing.T) {
	msgs, err := RenderTitleMaker(t.Context(), "| a | b |\n| --- | --- |\n| 1 | 2 |")
	require.NoError(t, err)
	require.NotEmpty(t, msgs)
	assert.Contains(t, msgs[0].Content, "| a | b |")
}

func TestDataTableCompensateRules(t *testing.T) {
	rules := dataTableCompensateRules(assert.AnError)
	require.NotEmpty(t, rules)
	assert.Contains(t, rules[len(rules)-1], assert.AnError.Error())
}
