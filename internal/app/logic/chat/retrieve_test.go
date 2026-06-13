package chat

import "testing"

func TestLLMRetrivalExpectTryExtract(t *testing.T) {
	content := `前置说明 {"foo":"bar"}，最终结果 {"intention":"纽约客2026年3月刊的栏目设置与主要文章内容概要","doc_ids":["2692e310918f49318b9bbfcd13fee63a"],"should_continue":true}`

	var expect llmRetrivalExpect
	extracted, ok := expect.tryExtract(content)
	if !ok {
		t.Fatalf("expect tryExtract to find json from text")
	}

	want := `{"intention":"纽约客2026年3月刊的栏目设置与主要文章内容概要","doc_ids":["2692e310918f49318b9bbfcd13fee63a"],"should_continue":true}`
	if extracted != want {
		t.Fatalf("unexpected extracted json: %s", extracted)
	}
}

func TestLLMRetrivalExpectTryExtract_NoTargetFields(t *testing.T) {
	content := `说明文本 {"foo":"bar"} {"title":"only metadata"}`

	var expect llmRetrivalExpect
	_, ok := expect.tryExtract(content)
	if ok {
		t.Fatalf("expect tryExtract to return not found for unrelated json")
	}
}

func TestLLMRetrivalExpectTryExtract_SkipUnrelatedCandidate(t *testing.T) {
	content := `先输出 {"foo":"bar"} 再输出 {"intention":"valid","doc_ids":["2692e310918f49318b9bbfcd13fee63a"]}`

	var expect llmRetrivalExpect
	extracted, ok := expect.tryExtract(content)
	if !ok {
		t.Fatalf("expect tryExtract to find valid candidate")
	}

	want := `{"intention":"valid","doc_ids":["2692e310918f49318b9bbfcd13fee63a"]}`
	if extracted != want {
		t.Fatalf("unexpected extracted json: %s", extracted)
	}
}
