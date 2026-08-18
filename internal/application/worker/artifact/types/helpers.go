package types

import (
	"fmt"
	"strings"

	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	einoschema "github.com/cloudwego/eino/schema"
)

const (
	logSnippetHead = 200
	logSnippetTail = 120
)

// TruncateForLog truncates noisy LLM output for slog fields (keeps head + tail).
func TruncateForLog(s string) string {
	return pkgstring.TruncateHeadTail(s, logSnippetHead, logSnippetTail)
}

func NormalizeStrings(sources []string) []string {
	normalized := make([]string, 0, len(sources))
	for _, source := range sources {
		text := strings.TrimSpace(source)
		if text == "" {
			continue
		}
		normalized = append(normalized, text)
	}

	return normalized
}

// BuildTipMessage constructs a user message for the rendered prompts.
// When tip is non-empty it carries the user extra requirement; when tip is empty
// it still returns a short user message so providers that require a user query
// accept the request, and the message stays in agent history.
func BuildTipMessage(tip string) *einoschema.Message {
	tip = strings.TrimSpace(tip)
	if tip == "" {
		return &einoschema.Message{
			Role:    einoschema.User,
			Content: "Please follow the system instructions and proceed.",
		}
	}

	return &einoschema.Message{
		Role:    einoschema.User,
		Content: "User extra requirement:\n<user_extra_input>\n" + tip + "\n</user_extra_input>",
	}
}

// BuildCompensateMessage constructs a user message that asks the LLM to re-output
// its result as strict JSON when the previous output failed parsing.
// fieldRules specifies the expected JSON fields and format constraints
func BuildCompensateMessage(output string, fieldRules []string) *einoschema.Message {
	rules := []string{"Output only one valid JSON object, without any explanatory text"}
	rules = append(rules, fieldRules...)
	rules = append(rules, "Do not wrap the output in ```json code fences")

	var b strings.Builder
	fmt.Fprintf(&b, "Your previous output does not meet the requirements. Please output it again strictly.\nCurrent output:\n%s\n\nRequirements:\n", output)
	for i, rule := range rules {
		fmt.Fprintf(&b, "%d) %s\n", i+1, rule)
	}

	return &einoschema.Message{
		Role:    einoschema.User,
		Content: b.String(),
	}
}

// BuildCompensatePlainMessage constructs a user message that asks the LLM to
// re-output plain text (non-JSON) under the given constraints.
func BuildCompensatePlainMessage(output string, fieldRules []string) *einoschema.Message {
	var b strings.Builder
	fmt.Fprintf(&b, "Your previous output does not meet the requirements. Please output it again strictly.\nCurrent output:\n%s\n\nRequirements:\n", output)
	for i, rule := range fieldRules {
		fmt.Fprintf(&b, "%d) %s\n", i+1, rule)
	}

	return &einoschema.Message{
		Role:    einoschema.User,
		Content: b.String(),
	}
}
