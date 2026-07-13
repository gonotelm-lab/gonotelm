package mindmap

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed mindmap.jinja
var mindmapPromptContent string

var mindmapTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(mindmapPromptContent))

type RenderVars struct {
	SourceIds []string
}

func (v RenderVars) promptVars() map[string]any {
	return map[string]any{
		"SourceIds": types.NormalizeStrings(v.SourceIds),
	}
}

func RenderMindmap(ctx context.Context, sourceIds []string) ([]*einoschema.Message, error) {
	msgs, err := mindmapTpl.Format(ctx, RenderVars{SourceIds: sourceIds}.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render mindmap prompt: %w", err)
	}
	return msgs, nil
}
