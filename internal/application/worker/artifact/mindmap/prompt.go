package mindmap

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed mindmap.jinja
var mindmapPromptContent string

var mindmapTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(mindmapPromptContent))

type RenderVars struct {
	SourceIds []string
	Tip       string
}

func (v RenderVars) promptVars() map[string]any {
	return map[string]any{
		"SourceIds": types.NormalizeStrings(v.SourceIds),
		"Tip":       v.Tip,
	}
}

func RenderMindmap(ctx context.Context, sourceIds []string, tip string) ([]*einoschema.Message, error) {
	msgs, err := mindmapTpl.Format(ctx, RenderVars{SourceIds: sourceIds, Tip: tip}.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render mindmap prompt: %w", err)
	}
	return msgs, nil
}
