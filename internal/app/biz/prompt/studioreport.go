package prompt

import ()

type StudioReportTemplateVars struct {
	SourceIds []string
}

func (v StudioReportTemplateVars) PromptVars() map[string]any {
	return map[string]any{
		"SourceIds": normalizeStrings(v.SourceIds),
	}
}

type StudioReportTemplate = template[StudioReportTemplateVars]

