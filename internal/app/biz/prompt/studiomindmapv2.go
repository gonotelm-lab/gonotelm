package prompt

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

type StudioMindmapV2TemplateVars struct {
	SourceIds []string
}

func (v StudioMindmapV2TemplateVars) PromptVars() map[string]any {
	return map[string]any{
		"SourceIds": normalizeStrings(v.SourceIds),
	}
}

type StudioMindmapV2Template = template[StudioMindmapV2TemplateVars]

func NewStudioMindmapV2Template(lang string) *StudioMindmapV2Template {
	return newTemplate[StudioMindmapV2TemplateVars](templateNameStudioMindmapV2, lang)
}

func RenderStudioMindmapV2Message(
	ctx context.Context,
	sourceIds []string,
	lang string,
) ([]*schema.Message, error) {
	tmpl := NewStudioMindmapV2Template(lang)
	msg, err := tmpl.Message(ctx, StudioMindmapV2TemplateVars{
		SourceIds: sourceIds,
	})
	if err != nil {
		return nil, err
	}

	return prependSystemMessage([]*schema.Message{msg}), nil
}
