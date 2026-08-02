package types

import (
	"fmt"
	"strings"

	einoschema "github.com/cloudwego/eino/schema"
)

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

// BuildTipMessage constructs an optional user message carrying the user-provided
// custom tip. It returns nil when the tip is empty, so callers can append it
// conditionally to the rendered system messages.
func BuildTipMessage(tip string) *einoschema.Message {
	tip = strings.TrimSpace(tip)
	if tip == "" {
		return nil
	}

	return &einoschema.Message{
		Role:    einoschema.User,
		Content: "用户附加要求：\n<user_extra_input>\n" + tip + "\n</user_extra_input>",
	}
}

// BuildCompensateMessage constructs a user message that asks the LLM to re-output
// its result as strict JSON when the previous output failed parsing.
// fieldRules specifies the expected JSON fields and format constraints
// (e.g. "JSON 字段必须且仅能包含 title 和 mindmap").
func BuildCompensateMessage(output string, fieldRules []string) *einoschema.Message {
	rules := []string{"只输出一个合法 JSON 对象，不要任何解释性文字"}
	rules = append(rules, fieldRules...)
	rules = append(rules, "不允许输出 ```json 代码块包裹")

	var b strings.Builder
	fmt.Fprintf(&b, "你刚才输出的结果不符合要求，请严格重输。\n当前输出：\n%s\n\n要求：\n", output)
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
	fmt.Fprintf(&b, "你刚才输出的结果不符合要求，请严格重输。\n当前输出：\n%s\n\n要求：\n", output)
	for i, rule := range fieldRules {
		fmt.Fprintf(&b, "%d) %s\n", i+1, rule)
	}

	return &einoschema.Message{
		Role:    einoschema.User,
		Content: b.String(),
	}
}
