package json

import (
	"testing"
)

type testMindmapPayload struct {
	Title   string `json:"title"`
	Mindmap string `json:"mindmap"`
}

type testPodcastOutline struct {
	Title    string              `json:"title"`
	Segments []testOutlineSegment `json:"segments"`
}

type testOutlineSegment struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func TestUnmarshal_DirectJSON(t *testing.T) {
	input := []byte(
		"{\"title\":\"Rust核心知识结构化思维导图\",\"mindmap\":\"```mermaid\\nmindmap\\n  root((Rust核心))\\n```\"}",
	)
	var got testMindmapPayload
	if err := Unmarshal(input, &got); err != nil {
		t.Fatalf("unmarshal direct json failed: %v", err)
	}
	if got.Title != "Rust核心知识结构化思维导图" {
		t.Fatalf("unexpected title: %s", got.Title)
	}
}

func TestUnmarshal_NoisyOutputWithJSONCodeBlock(t *testing.T) {
	input := []byte(
		"Now I have a good understanding of the source.\n" +
			"Let me create a structured mindmap.\n\n" +
			"```json\n" +
			"{\"title\":\"《纽约客》2026年3月2日刊核心内容导图\",\"mindmap\":\"```mermaid\\nmindmap\\n  root((《纽约客》032026刊))\\n    重要报道与调查\\n```\"}\n" +
			"```",
	)

	var got testMindmapPayload
	if err := Unmarshal(input, &got); err != nil {
		t.Fatalf("unmarshal noisy json failed: %v", err)
	}
	if got.Title != "《纽约客》2026年3月2日刊核心内容导图" {
		t.Fatalf("unexpected title: %s", got.Title)
	}
}

func TestUnmarshal_NoisyOutputWithRawJSONObject(t *testing.T) {
	input := []byte(
		"Thoughts before output...\n" +
			"{\"title\":\"Rust所有权学习导图\",\"mindmap\":\"```mermaid\\nmindmap\\n  root((Rust所有权))\\n```\"}",
	)

	var got testMindmapPayload
	if err := Unmarshal(input, &got); err != nil {
		t.Fatalf("unmarshal raw object failed: %v", err)
	}
	if got.Title != "Rust所有权学习导图" {
		t.Fatalf("unexpected title: %s", got.Title)
	}
}

func TestUnmarshalStrict_RejectUnknownField(t *testing.T) {
	input := []byte(
		"{\"title\":\"Rust所有权学习导图\",\"mindmap\":\"```mermaid\\nmindmap\\n  root((Rust所有权))\\n```\",\"extra\":\"x\"}",
	)

	var got testMindmapPayload
	if err := UnmarshalStrict(input, &got); err == nil {
		t.Fatalf("strict unmarshal should reject unknown fields")
	}
}

func TestUnmarshalStrict_FromNoisyOutput(t *testing.T) {
	input := []byte(
		"Thoughts before output...\n" +
			"```json\n" +
			"{\"title\":\"Rust所有权学习导图\",\"mindmap\":\"```mermaid\\nmindmap\\n  root((Rust所有权))\\n```\"}\n" +
			"```",
	)

	var got testMindmapPayload
	if err := UnmarshalStrict(input, &got); err != nil {
		t.Fatalf("strict unmarshal from noisy output failed: %v", err)
	}
	if got.Title != "Rust所有权学习导图" {
		t.Fatalf("unexpected title: %s", got.Title)
	}
}

func TestDecoder_LogOnDirectFailure_Called(t *testing.T) {
	input := []byte(
		"Reasoning...\n" +
			"```json\n" +
			"{\"title\":\"Rust所有权学习导图\",\"mindmap\":\"```mermaid\\nmindmap\\n  root((Rust所有权))\\n```\"}\n" +
			"```",
	)

	called := false
	decoder := Decoder{
		LogOnDirectFailure: func(err error, data []byte) {
			called = true
		},
	}

	var got testMindmapPayload
	if err := decoder.Unmarshal(input, &got); err != nil {
		t.Fatalf("decoder unmarshal failed: %v", err)
	}
	if !called {
		t.Fatalf("LogOnDirectFailure should be called when direct unmarshal fails")
	}
}

func TestDecoder_LogOnDirectFailure_NotCalledOnDirectSuccess(t *testing.T) {
	input := []byte(
		"{\"title\":\"Rust核心知识结构化思维导图\",\"mindmap\":\"```mermaid\\nmindmap\\n  root((Rust核心))\\n```\"}",
	)

	called := false
	decoder := Decoder{
		LogOnDirectFailure: func(err error, data []byte) {
			called = true
		},
	}

	var got testMindmapPayload
	if err := decoder.Unmarshal(input, &got); err != nil {
		t.Fatalf("decoder unmarshal failed: %v", err)
	}
	if called {
		t.Fatalf("LogOnDirectFailure should not be called when direct unmarshal succeeds")
	}
}

func TestFixUnescapedQuotes_NoChangeOnValidJSON(t *testing.T) {
	tests := []string{
		`{"title":"hello","segments":[{"name":"intro","content":"some text"}]}`,
		`{"title":"带中文的标题","segments":[]}`,
		`{"text":"hello \"world\""}`,
		`{"text":"line1\nline2"}`,
		`{"text":"path\\to\\file"}`,
		`{"text":""}`,
		`""`,
		``,
	}

	for _, tc := range tests {
		result := fixUnescapedQuotes([]byte(tc))
		if string(result) != tc {
			t.Errorf("input %q should not be modified, got %q", tc, string(result))
		}
	}
}

func TestFixUnescapedQuotes_EscapesUnescapedQuotesInStringValues(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `{"content":"新奥尔良"摔车"骗保帝"}`,
			expected: `{"content":"新奥尔良\"摔车\"骗保帝"}`,
		},
		{
			input:    `{"text":"他说"你好""}`,
			expected: `{"text":"他说\"你好\""}`,
		},
		{
			input:    `{"a":"b"c"d"}`,
			expected: `{"a":"b\"c\"d"}`,
		},
		{
			input:    `{"name":"intro","content":"some text with "quotes" inside"}`,
			expected: `{"name":"intro","content":"some text with \"quotes\" inside"}`,
		},
		{
			input:    `{"x":"start"middle"end"}`,
			expected: `{"x":"start\"middle\"end"}`,
		},
	}

	for _, tc := range tests {
		result := fixUnescapedQuotes([]byte(tc.input))
		if string(result) != tc.expected {
			t.Errorf("input:\n  %s\nexpected:\n  %s\ngot:\n  %s", tc.input, tc.expected, string(result))
		}
	}
}

func TestFixUnescapedQuotes_PreservesEscapedQuotes(t *testing.T) {
	input := `{"text":"already \"escaped\" content"}`
	result := fixUnescapedQuotes([]byte(input))
	if string(result) != input {
		t.Errorf("already escaped quotes should be preserved, got %q", string(result))
	}
}

func TestFixUnescapedQuotes_EmptyStringValues(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `{"a":""}`,
			expected: `{"a":""}`,
		},
		{
			input:    `{"a":"","b":"x"}`,
			expected: `{"a":"","b":"x"}`,
		},
	}

	for _, tc := range tests {
		result := fixUnescapedQuotes([]byte(tc.input))
		if string(result) != tc.expected {
			t.Errorf("input %q:\nexpected %q\ngot      %q", tc.input, tc.expected, string(result))
		}
	}
}

func TestFixUnescapedQuotes_ChineseCurlyQuotesUnaffected(t *testing.T) {
	inputRaw := "{\"text\":\"使用\xe2\x80\x9c中文弯引号\xe2\x80\x9d测试\"}"
	result := fixUnescapedQuotes([]byte(inputRaw))
	if string(result) != inputRaw {
		t.Errorf("Chinese curly quotes should not be affected, got %q", string(result))
	}
}

func TestUnmarshal_FixesUnescapedQuotes(t *testing.T) {
	input := []byte(`{"title":"播客标题","segments":[{"name":"开场","content":"新奥尔良"摔车"骗保帝事件"}]}`)
	var got testPodcastOutline
	if err := Unmarshal(input, &got); err != nil {
		t.Fatalf("unmarshal with unescaped quotes failed: %v", err)
	}
	if got.Title != "播客标题" {
		t.Fatalf("unexpected title: %s", got.Title)
	}
	if len(got.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(got.Segments))
	}
	expectedContent := `新奥尔良"摔车"骗保帝事件`
	if got.Segments[0].Content != expectedContent {
		t.Fatalf("expected content %q, got %q", expectedContent, got.Segments[0].Content)
	}
}

func TestUnmarshal_FixesUnescapedQuotesInCodeBlock(t *testing.T) {
	input := []byte("这是一些前置文字\n```json\n{\"title\":\"标题\",\"segments\":[{\"name\":\"片段1\",\"content\":\"使用\"和\"符号的文本\"}]}\n```\n后置文字")
	var got testPodcastOutline
	if err := Unmarshal(input, &got); err != nil {
		t.Fatalf("unmarshal unescaped quotes in code block failed: %v", err)
	}
	expectedContent := `使用"和"符号的文本`
	if got.Segments[0].Content != expectedContent {
		t.Fatalf("expected content %q, got %q", expectedContent, got.Segments[0].Content)
	}
}

func TestUnmarshal_FixesUnescapedQuotesInRawObject(t *testing.T) {
	input := []byte("前置思考文字\n{\"title\":\"标题\",\"segments\":[{\"name\":\"片段1\",\"content\":\"内容带\"引号\"\"}]}")
	var got testPodcastOutline
	if err := Unmarshal(input, &got); err != nil {
		t.Fatalf("unmarshal unescaped quotes in raw object failed: %v", err)
	}
	expectedContent := `内容带"引号"`
	if got.Segments[0].Content != expectedContent {
		t.Fatalf("expected content %q, got %q", expectedContent, got.Segments[0].Content)
	}
}
