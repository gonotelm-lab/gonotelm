package openai

var StreamOptionsIncludeUsage = map[string]any{
	"stream_options": map[string]bool{
		"include_usage": true,
	},
}

var ResponseFormatJSONObject = map[string]any{
	"response_format": map[string]string{
		"type": "json_object",
	},
}
