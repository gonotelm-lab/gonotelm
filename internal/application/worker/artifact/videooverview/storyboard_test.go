package videooverview

import (
	"context"
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testOutline = &videoOutline{
	Title: "机器学习入门",
	Segments: []videoOutlineSegment{
		{Name: "开场", Content: "介绍机器学习是什么"},
		{Name: "总结", Content: "回顾核心结论"},
	},
}

func TestParseOutlineOutput_HappyPath(t *testing.T) {
	content := `{
		"title": "机器学习入门",
		"segments": [
			{"name": "开场", "content": "介绍机器学习是什么"},
			{"name": "总结", "content": "回顾核心结论"}
		]
	}`

	outline, err := (&outlineGenerator{}).parse(context.Background(), content)
	require.NoError(t, err)
	assert.Equal(t, "机器学习入门", outline.Title)
	require.Len(t, outline.Segments, 2)
	assert.Equal(t, "开场", outline.Segments[0].Name)
}

func TestParseOutlineOutput_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"empty output", ""},
		{"empty title", `{"title":"  ","segments":[{"name":"s","content":"c"}]}`},
		{"empty segments", `{"title":"t","segments":[]}`},
		{"segment name empty", `{"title":"t","segments":[{"name":" ","content":"c"}]}`},
		{"segment content empty", `{"title":"t","segments":[{"name":"s","content":"  "}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&outlineGenerator{}).parse(context.Background(), tt.content)
			assert.Error(t, err)
		})
	}
}

func TestParseOutlineOutput_TooManySegments(t *testing.T) {
	segments := ""
	for range maxVideoSegments + 1 {
		segments += `{"name":"s","content":"c"},`
	}
	content := `{"title":"t","segments":[` + segments[:len(segments)-1] + `]}`
	_, err := (&outlineGenerator{}).parse(context.Background(), content)
	assert.Error(t, err)
}

func TestParseStoryboardOutput_HappyPath(t *testing.T) {
	content := `{
		"title": "机器学习入门",
		"scenes": [
			{"name": "开场", "narration": "大家好，今天我们来聊聊机器学习。", "visual": {"headline": "机器学习是什么", "points": ["定义", "应用"], "layout": "bullets"}},
			{"name": "总结", "narration": "以上就是机器学习的核心内容。", "visual": {"headline": "回顾", "points": ["结论"], "layout": "quote"}}
		]
	}`

	sb, err := (&storyboardGenerator{}).parse(context.Background(), content, testOutline)
	require.NoError(t, err)
	// title 以大纲为准
	assert.Equal(t, testOutline.Title, sb.Title)
	require.Len(t, sb.Scenes, 2)
	assert.Equal(t, "开场", sb.Scenes[0].Name)
	assert.Equal(t, "bullets", sb.Scenes[0].Visual.Layout)
}

func TestParseStoryboardOutput_StripJSONPrefix(t *testing.T) {
	content := "```json\n{\"title\":\"t\",\"scenes\":[{\"name\":\"s\",\"narration\":\"n\",\"visual\":{\"headline\":\"h\"}}]}\n```"
	singleSegOutline := &videoOutline{
		Title:    "机器学习入门",
		Segments: []videoOutlineSegment{{Name: "开场", Content: "介绍机器学习是什么"}},
	}
	sb, err := (&storyboardGenerator{}).parse(context.Background(), content, singleSegOutline)
	require.NoError(t, err)
	assert.Equal(t, singleSegOutline.Title, sb.Title)
	require.Len(t, sb.Scenes, 1)
}

func TestParseStoryboardOutput_NarrationWhitespaceCollapsed(t *testing.T) {
	content := `{"title":"t","scenes":[{"name":"s","narration":"第一句。\n\n第二句。","visual":{"headline":"h"}}]}`
	singleSegOutline := &videoOutline{
		Title:    "机器学习入门",
		Segments: []videoOutlineSegment{{Name: "开场", Content: "介绍机器学习是什么"}},
	}
	sb, err := (&storyboardGenerator{}).parse(context.Background(), content, singleSegOutline)
	require.NoError(t, err)
	assert.Equal(t, "第一句。 第二句。", sb.Scenes[0].Narration)
}

func TestParseStoryboardOutput_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"empty output", ""},
		{"empty title", `{"title":"  ","scenes":[{"name":"s","narration":"n","visual":{"headline":"h"}}]}`},
		{"empty scenes", `{"title":"t","scenes":[]}`},
		{"scene narration empty", `{"title":"t","scenes":[{"name":"s","narration":"  ","visual":{"headline":"h"}}]}`},
		{"scene name empty", `{"title":"t","scenes":[{"name":" ","narration":"n","visual":{"headline":"h"}}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&storyboardGenerator{}).parse(context.Background(), tt.content, testOutline)
			assert.Error(t, err)
		})
	}
}

func TestParseStoryboardOutput_TooManyScenes(t *testing.T) {
	scenes := ""
	for range maxVideoScenes + 1 {
		scenes += `{"name":"s","narration":"n","visual":{"headline":"h"}},`
	}
	content := `{"title":"t","scenes":[` + scenes[:len(scenes)-1] + `]}`
	_, err := (&storyboardGenerator{}).parse(context.Background(), content, testOutline)
	assert.Error(t, err)
}

func TestParseStoryboardOutput_ScenesLessThanSegments(t *testing.T) {
	outline := &videoOutline{
		Title: "t",
		Segments: []videoOutlineSegment{
			{Name: "a", Content: "a"},
			{Name: "b", Content: "b"},
			{Name: "c", Content: "c"},
		},
	}
	content := `{"title":"t","scenes":[{"name":"s1","narration":"n","visual":{"headline":"h"}},{"name":"s2","narration":"n","visual":{"headline":"h"}}]}`
	_, err := (&storyboardGenerator{}).parse(context.Background(), content, outline)
	assert.Error(t, err)
}

func TestParseStoryboardOutput_TooLongNarration(t *testing.T) {
	long := strings.Repeat("字", maxNarrationLength+1)
	content := `{"title":"t","scenes":[{"name":"s","narration":"` + long + `","visual":{"headline":"h"}}]}`
	_, err := (&storyboardGenerator{}).parse(context.Background(), content, testOutline)
	assert.Error(t, err)
}

func TestResolveNarratorVoice(t *testing.T) {
	t.Run("qwen zh", func(t *testing.T) {
		v, err := resolveNarratorVoice(text2audio.Text2AudioQwen, entity.LanguageChinese)
		require.NoError(t, err)
		assert.NotEmpty(t, v)
	})
	t.Run("mimo en", func(t *testing.T) {
		v, err := resolveNarratorVoice(text2audio.Text2AudioMimo, entity.LanguageEnglish)
		require.NoError(t, err)
		assert.NotEmpty(t, v)
	})
	t.Run("unknown provider", func(t *testing.T) {
		_, err := resolveNarratorVoice(text2audio.Text2AudioProvider("bogus"), entity.LanguageChinese)
		assert.Error(t, err)
	})
}
