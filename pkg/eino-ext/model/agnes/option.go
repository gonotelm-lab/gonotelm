package agnes

import (
	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
)

// WithExtraFields returns a request-level option that merges the provided
// key/value pairs into the top-level JSON payload sent to the Agnes API.
//
// Keys that collide with fields already populated by this component (e.g.
// "model", "messages", "temperature", ...) will override them.
//
// Passing a nil or empty map is a no-op.
//
// Example:
//
//	msg, err := cm.Generate(ctx, in,
//	    agnes.WithExtraFields(map[string]any{
//	        "chat_template_kwargs": map[string]any{
//	            "thinking": true,
//	        },
//	    }),
//	)
func WithExtraFields(extraFields map[string]any) model.Option {
	return openai.WithExtraFields(extraFields)
}
