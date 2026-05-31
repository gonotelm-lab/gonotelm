package indices

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unsafe"

	einoembed "github.com/cloudwego/eino/components/embedding"
	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	. "github.com/smartystreets/goconvey/convey"
)

type parseBuildMockEmbedder struct {
	mu           sync.Mutex
	calls        [][]string
	returnErr    error
	resultShrink int
}

func (m *parseBuildMockEmbedder) EmbedStrings(
	_ context.Context,
	texts []string,
	_ ...einoembed.Option,
) ([][]float64, error) {
	m.mu.Lock()
	m.calls = append(m.calls, append([]string(nil), texts...))
	returnErr := m.returnErr
	resultShrink := m.resultShrink
	m.mu.Unlock()

	if returnErr != nil {
		return nil, returnErr
	}

	resultCount := len(texts) - resultShrink
	if resultCount < 0 {
		resultCount = 0
	}
	embeddings := make([][]float64, resultCount)
	for i := 0; i < resultCount; i++ {
		text := texts[i]
		embeddings[i] = []float64{float64(len([]rune(text))), float64(i + 1)}
	}
	return embeddings, nil
}

func (m *parseBuildMockEmbedder) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

type parseBuildMockLLM struct {
	mu            sync.Mutex
	response      string
	generateErr   error
	generateCalls int
	inputs        []string
}

func (m *parseBuildMockLLM) Generate(
	_ context.Context,
	input []*einoschema.Message,
	_ ...einomodel.Option,
) (*einoschema.Message, error) {
	var sb strings.Builder
	for _, msg := range input {
		if msg == nil {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n---\n")
		}
		sb.WriteString(msg.Content)
	}

	m.mu.Lock()
	m.generateCalls++
	m.inputs = append(m.inputs, sb.String())
	generateErr := m.generateErr
	m.mu.Unlock()

	if generateErr != nil {
		return nil, generateErr
	}

	return &einoschema.Message{
		Role:    einoschema.Assistant,
		Content: m.response,
	}, nil
}

func (m *parseBuildMockLLM) Stream(
	_ context.Context,
	_ []*einoschema.Message,
	_ ...einomodel.Option,
) (*einoschema.StreamReader[*einoschema.Message], error) {
	return nil, nil
}

func (m *parseBuildMockLLM) WithTools(
	_ []*einoschema.ToolInfo,
) (einomodel.ToolCallingChatModel, error) {
	return m, nil
}

func (m *parseBuildMockLLM) GenerateCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generateCalls
}

func (m *parseBuildMockLLM) Inputs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.inputs...)
}

func newParseBuildMockGateway(
	providerType chat.Type,
	provider einomodel.ToolCallingChatModel,
) *gateway.Gateway {
	gw := &gateway.Gateway{}
	providers := map[chat.Type]einomodel.ToolCallingChatModel{
		providerType: provider,
	}

	// test helper: inject mock providers into unexported field.
	rv := reflect.ValueOf(gw).Elem().FieldByName("providers")
	reflect.NewAt(rv.Type(), unsafe.Pointer(rv.UnsafeAddr())).Elem().Set(reflect.ValueOf(providers))
	return gw
}

func parseBuildSplitByRuneWindow(maxSize int) ParseBuildChunkSplitFunc {
	return func(_ context.Context, content string) ([]string, error) {
		if maxSize <= 0 || content == "" {
			return []string{content}, nil
		}
		runes := []rune(content)
		if len(runes) <= maxSize {
			return []string{content}, nil
		}

		chunks := make([]string, 0, len(runes)/maxSize+1)
		for start := 0; start < len(runes); {
			end := start + maxSize
			if end > len(runes) {
				end = len(runes)
			}
			chunks = append(chunks, string(runes[start:end]))
			start = end
		}
		return chunks, nil
	}
}

func parseBuildRuneTokenLen(text string) int {
	return len([]rune(text))
}

func collectTitleNodes(node *DocTreeNode, title string) []*DocTreeNode {
	if node == nil {
		return nil
	}
	nodes := make([]*DocTreeNode, 0)
	if node.Core() != nil {
		content := node.Core().Content
		firstLine := content
		if idx := strings.Index(firstLine, "\n"); idx >= 0 {
			firstLine = firstLine[:idx]
		}
		if strings.TrimSpace(firstLine) == title {
			nodes = append(nodes, node)
		}
	}
	for _, child := range node.Children() {
		nodes = append(nodes, collectTitleNodes(child, title)...)
	}
	return nodes
}

func findNodeByExactContent(node *DocTreeNode, content string) *DocTreeNode {
	if node == nil {
		return nil
	}
	if node.Core() != nil && node.Core().Content == content {
		return node
	}
	for _, child := range node.Children() {
		if found := findNodeByExactContent(child, content); found != nil {
			return found
		}
	}
	return nil
}

func assertAllNodesEmbedded(nodes []*DocTreeNode) {
	for _, node := range nodes {
		So(node, ShouldNotBeNil)
		So(node.Core(), ShouldNotBeNil)
		So(len(node.Core().Embedding), ShouldBeGreaterThan, 0)
	}
}

func TestParseBuildMockedLeafSplit(t *testing.T) {
	Convey("ParseBuild 叶子节点超限时应横向分裂", t, func() {
		mockLLM := &parseBuildMockLLM{response: "ROOT-MOCK"}
		mockEmbedder := &parseBuildMockEmbedder{}
		mockGateway := newParseBuildMockGateway(chat.Openai, mockLLM)

		builder := NewDocTreeBuilder(
			mockEmbedder,
			mockGateway,
			func(_ context.Context) string { return string(chat.Openai) },
			func(_ context.Context) string { return "mock-model" },
		)

		markdown := strings.Join([]string{
			"# Leaf",
			strings.Repeat("leaf-content-", 24),
		}, "\n")

		tree, err := builder.ParseBuild(
			context.Background(),
			[]byte(markdown),
			WithParseBuildMaxNodeToken(36),
			WithParseBuildTokenLenFn(parseBuildRuneTokenLen),
			WithParseBuildChunkSplitFunc(parseBuildSplitByRuneWindow(24)),
		)

		So(err, ShouldBeNil)
		So(tree, ShouldNotBeNil)
		So(tree.Root(), ShouldNotBeNil)
		So(tree.Root().Core(), ShouldNotBeNil)
		So(tree.Root().Core().Content, ShouldEqual, "ROOT-MOCK")
		So(mockLLM.GenerateCallCount(), ShouldEqual, 1)

		inputs := mockLLM.Inputs()
		So(len(inputs), ShouldBeGreaterThan, 0)
		So(inputs[0], ShouldContainSubstring, "Leaf")

		leafNodes := collectTitleNodes(tree.Root(), "Leaf")
		So(len(leafNodes), ShouldBeGreaterThan, 1)

		assertAllNodesEmbedded(tree.Nodes())
		So(mockEmbedder.CallCount(), ShouldBeGreaterThan, 0)
	})
}

func TestParseBuildMockedNonLeafDownshift(t *testing.T) {
	Convey("ParseBuild 非叶子节点超限时应内容下放", t, func() {
		mockLLM := &parseBuildMockLLM{response: "ROOT-NON-LEAF"}
		mockEmbedder := &parseBuildMockEmbedder{}
		mockGateway := newParseBuildMockGateway(chat.Openai, mockLLM)

		builder := NewDocTreeBuilder(
			mockEmbedder,
			mockGateway,
			func(_ context.Context) string { return string(chat.Openai) },
			func(_ context.Context) string { return "mock-model" },
		)

		markdown := strings.Join([]string{
			"# Parent",
			strings.Repeat("parent-content-", 20),
			"## Child",
			"child-body",
		}, "\n")

		tree, err := builder.ParseBuild(
			context.Background(),
			[]byte(markdown),
			WithParseBuildMaxNodeToken(40),
			WithParseBuildTokenLenFn(parseBuildRuneTokenLen),
			WithParseBuildChunkSplitFunc(parseBuildSplitByRuneWindow(22)),
		)
		So(err, ShouldBeNil)
		So(tree, ShouldNotBeNil)
		So(tree.Root(), ShouldNotBeNil)

		parentNode := findNodeByExactContent(tree.Root(), "Parent")
		So(parentNode, ShouldNotBeNil)

		// 非叶子超限后，父节点仅保留标题；内容下放为子节点（无标题纯内容节点）。
		hasDownshiftContent := false
		hasChildSection := false
		for _, child := range parentNode.Children() {
			if child == nil || child.Core() == nil {
				continue
			}
			content := child.Core().Content
			if content == "" {
				continue
			}
			if content == "Child\nchild-body" {
				hasChildSection = true
				continue
			}
			if !strings.Contains(content, "\n") && strings.Contains(content, "parent-content-") {
				hasDownshiftContent = true
			}
		}
		So(hasDownshiftContent, ShouldBeTrue)
		So(hasChildSection, ShouldBeTrue)

		assertAllNodesEmbedded(tree.Nodes())
		So(mockLLM.GenerateCallCount(), ShouldEqual, 1)
	})
}

func TestParseBuildMockedLLMError(t *testing.T) {
	Convey("ParseBuild 在 vroot 标题生成失败时应返回错误", t, func() {
		mockLLM := &parseBuildMockLLM{
			response:    "ROOT-ERR",
			generateErr: errors.New("mock llm error"),
		}
		mockEmbedder := &parseBuildMockEmbedder{}
		mockGateway := newParseBuildMockGateway(chat.Openai, mockLLM)

		builder := NewDocTreeBuilder(
			mockEmbedder,
			mockGateway,
			func(_ context.Context) string { return string(chat.Openai) },
			func(_ context.Context) string { return "mock-model" },
		)

		markdown := "# Title\nbody"
		tree, err := builder.ParseBuild(
			context.Background(),
			[]byte(markdown),
			WithParseBuildMaxNodeToken(40),
			WithParseBuildTokenLenFn(parseBuildRuneTokenLen),
			WithParseBuildChunkSplitFunc(parseBuildSplitByRuneWindow(20)),
		)

		So(tree, ShouldBeNil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "generate vroot title failed")
		So(mockLLM.GenerateCallCount(), ShouldEqual, 1)
		So(mockEmbedder.CallCount(), ShouldEqual, 0)
	})
}

func TestParseBuildMockedEmbedCountMismatch(t *testing.T) {
	Convey("ParseBuild 在 embedding 数量不匹配时应返回错误", t, func() {
		mockLLM := &parseBuildMockLLM{response: "ROOT-OK"}
		mockEmbedder := &parseBuildMockEmbedder{resultShrink: 1}
		mockGateway := newParseBuildMockGateway(chat.Openai, mockLLM)

		builder := NewDocTreeBuilder(
			mockEmbedder,
			mockGateway,
			func(_ context.Context) string { return string(chat.Openai) },
			func(_ context.Context) string { return "mock-model" },
		)

		markdown := strings.Join([]string{
			"# Parent",
			"parent body",
			"## Child",
			"child body",
		}, "\n")

		tree, err := builder.ParseBuild(
			context.Background(),
			[]byte(markdown),
			WithParseBuildMaxNodeToken(80),
			WithParseBuildTokenLenFn(parseBuildRuneTokenLen),
			WithParseBuildChunkSplitFunc(parseBuildSplitByRuneWindow(40)),
		)

		So(tree, ShouldBeNil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "embed parse-build nodes failed")
		So(mockLLM.GenerateCallCount(), ShouldEqual, 1)
		So(mockEmbedder.CallCount(), ShouldBeGreaterThan, 0)
	})
}
