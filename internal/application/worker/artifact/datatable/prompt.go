package datatable

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed datatable.jinja
var dataTablePromptContent string

var dataTableTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(dataTablePromptContent))

type RenderVars struct {
	SourceIds []string
}

func (v RenderVars) promptVars() map[string]any {
	return map[string]any{
		"SourceIds": types.NormalizeStrings(v.SourceIds),
	}
}

func RenderDataTable(ctx context.Context, sourceIds []string, tip string) ([]*einoschema.Message, error) {
	msgs, err := dataTableTpl.Format(ctx, RenderVars{SourceIds: sourceIds}.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render datatable prompt: %w", err)
	}
	if tipMsg := types.BuildTipMessage(tip); tipMsg != nil {
		msgs = append(msgs, tipMsg)
	}
	return msgs, nil
}
