package prompt

import ()

type StudioMindmapV2TemplateVars struct {
	SourceIds []string
}

func (v StudioMindmapV2TemplateVars) PromptVars() map[string]any {
	return map[string]any{
		"SourceIds": normalizeStrings(v.SourceIds),
	}
}

type StudioMindmapV2Template = template[StudioMindmapV2TemplateVars]

