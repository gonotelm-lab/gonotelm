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

// BuildCompensateMessage asks the LLM to re-output as strict JSON on parse failure.
// The previous output is not replayed (already in agent context); only the step
// duty and the failure reasons are injected.
func BuildCompensateMessage(duty string, fieldRules []string) *einoschema.Message {
	rules := []string{"Output only one valid JSON object, without any explanatory text"}
	rules = append(rules, fieldRules...)
	rules = append(rules, "Do not wrap the output in ```json code fences")
	return buildCompensateMessage(duty, rules)
}

// BuildCompensatePlainMessage is the non-JSON variant of BuildCompensateMessage.
func BuildCompensatePlainMessage(duty string, fieldRules []string) *einoschema.Message {
	return buildCompensateMessage(duty, fieldRules)
}

func buildCompensateMessage(duty string, rules []string) *einoschema.Message {
	var b strings.Builder
	b.WriteString("Your previous output does not meet the requirements. Please output it again strictly.")
	if duty = strings.TrimSpace(duty); duty != "" {
		fmt.Fprintf(&b, "\n\nYour duty: %s", duty)
	}
	b.WriteString("\n\nRequirements:\n")
	for i, rule := range rules {
		fmt.Fprintf(&b, "%d) %s\n", i+1, rule)
	}

	return &einoschema.Message{
		Role:    einoschema.User,
		Content: b.String(),
	}
}
