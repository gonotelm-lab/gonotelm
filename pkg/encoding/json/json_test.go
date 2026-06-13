package json

import "testing"

type testMindmapPayload struct {
	Title   string `json:"title"`
	Mindmap string `json:"mindmap"`
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
