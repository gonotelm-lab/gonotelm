package string

import (
	"testing"
)

func TestExtractBetweenTags(t *testing.T) {
	tests := []struct {
		s        string
		openTag  string
		closeTag string
		expected string
	}{
		{
			s:        "<thinking_summary>This is a test</thinking_summary>",
			openTag:  "<thinking_summary>",
			closeTag: "</thinking_summary>",
			expected: "This is a test",
		},
		{
			s:        "<code>func main() {\n\tfmt.Println(1)\n}</code>",
			openTag:  "<code>",
			closeTag: "</code>",
			expected: "func main() {\n\tfmt.Println(1)\n}",
		},
		{
			s:        "<tag></tag>",
			openTag:  "<tag>",
			closeTag: "</tag>",
			expected: "",
		},
		{
			s:        "This is a test</tag>",
			openTag:  "<tag>",
			closeTag: "</tag>",
			expected: "",
		},
		{
			s:        "<tag>This is a test",
			openTag:  "<tag>",
			closeTag: "</tag>",
			expected: "",
		},
		{
			s:        "<t>a</t> some text <t>b</t>",
			openTag:  "<t>",
			closeTag: "</t>",
			expected: "a",
		},
		{
			s:        "prefix <tag>content</tag> suffix",
			openTag:  "<tag>",
			closeTag: "</tag>",
			expected: "content",
		},
		{
			s:        "<pre>if (a < b && c > d) { return true; }</pre>",
			openTag:  "<pre>",
			closeTag: "</pre>",
			expected: "if (a < b && c > d) { return true; }",
		},
	}

	for _, tt := range tests {
		result := ExtractBetweenTags(tt.s, tt.openTag, tt.closeTag)
		if result != tt.expected {
			t.Errorf("expect %s, but got %s", tt.expected, result)
		}
	}
}
