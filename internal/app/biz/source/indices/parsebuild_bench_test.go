package indices

import (
	"context"
	"fmt"
	"strings"
	"testing"

	sourceutil "github.com/gonotelm-lab/gonotelm/internal/app/biz/source/util"
	"github.com/gonotelm-lab/gonotelm/pkg/token"
	"github.com/yuin/goldmark"
	goldtext "github.com/yuin/goldmark/text"
)

// generateBenchMarkdown builds synthetic markdown with the specified number
// of headings and approximate paragraph size for reproducible benchmarks.
func generateBenchMarkdown(headings, paragraphRunes int) []byte {
	var sb strings.Builder
	for i := 0; i < headings; i++ {
		level := (i % 3) + 1 // cycle H1-H3
		sb.WriteString(strings.Repeat("#", level))
		sb.WriteString(fmt.Sprintf(" Section %d\n\n", i))

		sentence := "这是一段用于性能测试的示例文本，包含中文和English混合内容。"
		repeats := paragraphRunes / len([]rune(sentence))
		if repeats < 1 {
			repeats = 1
		}
		for j := 0; j < repeats; j++ {
			sb.WriteString(sentence)
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n')
	}
	return []byte(sb.String())
}

func newBenchBuilder() *DocTreeBuilder {
	return NewDocTreeBuilder(
		&parseBuildMockEmbedder{},
		&parseBuildMockLLM{response: "BENCH-ROOT"},
	)
}

// --- ParseBuild end-to-end (mocked LLM/embed) ---

func BenchmarkParseBuild(b *testing.B) {
	cases := []struct {
		name             string
		headings         int
		paragraphRunes   int
	}{
		{"small_1KB", 3, 40},
		{"medium_10KB", 20, 60},
		{"large_100KB", 100, 120},
	}

	for _, tc := range cases {
		md := generateBenchMarkdown(tc.headings, tc.paragraphRunes)
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(md)))
			b.ReportAllocs()
			ctx := context.Background()
			for b.Loop() {
				builder := newBenchBuilder()
				_, err := builder.ParseBuild(ctx, md)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// --- Sub-function benchmarks ---

func BenchmarkBuildRuneIndexByByteOffset(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
	}
	for _, sz := range sizes {
		sentence := "这是性能测试的中文和English混合文本。"
		reps := sz.size / len(sentence)
		if reps < 1 {
			reps = 1
		}
		src := strings.Repeat(sentence, reps)

		b.Run(sz.name, func(b *testing.B) {
			b.SetBytes(int64(len(src)))
			b.ReportAllocs()
			for b.Loop() {
				sourceutil.BuildRuneIndexByByteOffset(src)
			}
		})
	}
}

func BenchmarkTokenEstimate(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"100B", 100},
		{"1KB", 1024},
		{"10KB", 10 * 1024},
	}
	for _, sz := range sizes {
		sentence := "这是token计数性能测试的文本。Hello world! 12345."
		reps := sz.size / len(sentence)
		if reps < 1 {
			reps = 1
		}
		text := strings.Repeat(sentence, reps)

		b.Run(sz.name, func(b *testing.B) {
			b.SetBytes(int64(len(text)))
			b.ReportAllocs()
			for b.Loop() {
				token.Estimate(text)
			}
		})
	}
}

func BenchmarkExtractBlockText(b *testing.B) {
	md := []byte(strings.Join([]string{
		"这是一段普通文本。",
		"",
		"```go",
		"func main() { fmt.Println(\"hello\") }",
		"```",
		"",
		"- 列表项一",
		"- 列表项二",
		"- 列表项三",
		"",
		"> 引用块内容",
		"> 第二行引用",
	}, "\n"))

	parser := goldmark.DefaultParser()
	reader := goldtext.NewReader(md)
	doc := parser.Parse(reader)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
			extractMarkdownBlockText(n, md)
		}
	}
}

func BenchmarkNormalizeChunks(b *testing.B) {
	chunks := make([]ParseBuildChunk, 50)
	for i := range chunks {
		chunks[i] = ParseBuildChunk{
			Content:   fmt.Sprintf("\n\nchunk content %d with some text\n\n", i),
			StartByte: i * 100,
			EndByte:   i*100 + 80,
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		normalizeChunks(chunks)
	}
}

func BenchmarkJoinedOffsetToSourceByte(b *testing.B) {
	contents := make([]string, 20)
	ranges := make([]markdownByteRange, 20)
	offset := 0
	for i := range contents {
		contents[i] = fmt.Sprintf("content block number %d with moderate length text", i)
		ranges[i] = markdownByteRange{start: offset, end: offset + len(contents[i]) + 10}
		offset = ranges[i].end + 5
	}

	totalLen := 0
	for i, c := range contents {
		totalLen += len(c)
		if i < len(contents)-1 {
			totalLen++
		}
	}
	testOffset := totalLen / 2

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		joinedOffsetToSourceByte(contents, ranges, testOffset, false)
	}
}

func BenchmarkBuildChunkByteSpans(b *testing.B) {
	sentence := "这是用于测试chunk定位的文本段落。"
	source := strings.Repeat(sentence, 100)

	chunks := make([]string, 20)
	chunkLen := len(source) / 20
	for i := range chunks {
		start := i * chunkLen
		end := start + chunkLen
		if end > len(source) {
			end = len(source)
		}
		chunks[i] = source[start:end]
	}

	b.SetBytes(int64(len(source)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		sourceutil.BuildChunkByteSpans(source, chunks)
	}
}

func BenchmarkAlignByteRangeByContent(b *testing.B) {
	source := []byte(strings.Repeat("这是对齐测试的文本内容。Hello alignment test. ", 50))
	content := "Hello alignment test."
	br := markdownByteRange{start: 0, end: len(source)}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		alignByteRangeByContent(content, br, source)
	}
}

func BenchmarkCollectNodeTitles(b *testing.B) {
	root := &markdownDocTreeNode{title: "root"}
	for i := 0; i < 20; i++ {
		child := &markdownDocTreeNode{title: fmt.Sprintf("Section %d", i)}
		for j := 0; j < 5; j++ {
			grandchild := &markdownDocTreeNode{title: fmt.Sprintf("Sub %d.%d", i, j)}
			child.children = append(child.children, grandchild)
		}
		root.children = append(root.children, child)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		collectNodeTitles(root.children)
	}
}
