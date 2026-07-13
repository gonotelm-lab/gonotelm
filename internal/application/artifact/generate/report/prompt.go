package report

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed report.jinja
var reportPromptContent string

var reportTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(reportPromptContent))

type RenderVars struct {
	SourceIds []string
}

func (v RenderVars) promptVars() map[string]any {
	return map[string]any{
		"SourceIds": types.NormalizeStrings(v.SourceIds),
	}
}

func RenderReport(ctx context.Context, sourceIds []string) ([]*einoschema.Message, error) {
	msgs, err := reportTpl.Format(ctx, RenderVars{SourceIds: sourceIds}.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render report prompt: %w", err)
	}
	return msgs, nil
}
