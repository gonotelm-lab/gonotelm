package chat

import openaiext "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/openai"

var streamOptionsIncludeUsage = openaiext.StreamOptionsIncludeUsage

func cloneExtraFields(base map[string]any) map[string]any {
	if len(base) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(base))
	for k, v := range base {
		out[k] = v
	}
	return out
}

// mergeExtraFields returns a new map with overrides applied on top of base.
// eino-ext WithExtraFields replaces the whole map, so callers must merge explicitly.
func mergeExtraFields(base map[string]any, overrides map[string]any) map[string]any {
	out := cloneExtraFields(base)
	for k, v := range overrides {
		out[k] = v
	}
	return out
}
