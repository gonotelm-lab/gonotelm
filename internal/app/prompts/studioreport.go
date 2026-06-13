package prompts

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

type StudioReportTemplateVars struct {
	SourceIds []string
}

func (v StudioReportTemplateVars) PromptVars() map[string]any {
	return map[string]any{
		"SourceIds": normalizeStrings(v.SourceIds),
	}
}

type StudioReportTemplate = template[StudioReportTemplateVars]

func NewStudioReportTemplate(lang string) *StudioReportTemplate {
	return newTemplate[StudioReportTemplateVars](templateNameStudioReport, lang)
}

func RenderStudioReportMessage(
	ctx context.Context,
	sourceIds []string,
	lang string,
) ([]*schema.Message, error) {
	tmpl := NewStudioReportTemplate(lang)
	msg, err := tmpl.Message(ctx, StudioReportTemplateVars{
		SourceIds: sourceIds,
	})
	if err != nil {
		return nil, err
	}

	return prependSystemMessage([]*schema.Message{msg}), nil
}
