package recursive

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	einorecursive "github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/token"
)

var benchmarkSink int

func BenchmarkRecursiveSplitterComparison(b *testing.B) {
	ctx := context.Background()
	chunkSize := 500
	overlapSize := 75
	separators := []string{"\n\n", "\n", ". ", "? ", "! ", "。", "？", "！", "，", ", "}
	lenFunc := token.EstimateToken

	cases := []struct {
		name    string
		content string
	}{
		{name: "english_short_sentences", content: buildEnglishBenchmarkContent()},
		{name: "chinese_rust_tutorial", content: buildChineseBenchmarkContent()},
		{name: "markdown_sections", content: buildMarkdownBenchmarkContent()},
		{name: "long_sentence_pressure", content: buildLongSentenceBenchmarkContent()},
		{name: "one_million_text", content: buildOneMillionBenchmarkContent()},
	}

	for _, tc := range cases {
		docs := []*schema.Document{
			{
				ID:      "benchmark_doc",
				Content: tc.content,
				MetaData: map[string]any{
					"source": "benchmark",
				},
			},
		}

		b.Run(tc.name+"/gonotelm_recursive_sentence_window", func(b *testing.B) {
			transformer, err := NewSplitter(ctx, &Config{
				ChunkSize:   chunkSize,
				OverlapSize: overlapSize,
				Separators:  separators,
				LenFunc:     lenFunc,
				KeepType:    KeepTypeEnd,
			})
			if err != nil {
				b.Fatal(err)
			}

			benchmarkTransform(ctx, b, transformer, docs)
		})

		b.Run(tc.name+"/eino_recursive", func(b *testing.B) {
			transformer, err := einorecursive.NewSplitter(ctx, &einorecursive.Config{
				ChunkSize:   chunkSize,
				OverlapSize: overlapSize,
				Separators:  separators,
				LenFunc:     lenFunc,
				KeepType:    einorecursive.KeepTypeEnd,
			})
			if err != nil {
				b.Fatal(err)
			}

			benchmarkTransform(ctx, b, transformer, docs)
		})
	}
}

func benchmarkTransform(ctx context.Context, b *testing.B, transformer document.Transformer, docs []*schema.Document) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()

	total := 0
	for i := 0; i < b.N; i++ {
		out, err := transformer.Transform(ctx, docs)
		if err != nil {
			b.Fatal(err)
		}
		total += len(out)
		if len(out) > 0 {
			total += len(out[0].Content)
		}
	}

	benchmarkSink = total
}

func buildEnglishBenchmarkContent() string {
	var builder strings.Builder
	builder.Grow(256 * 1024)

	for i := 0; i < 1200; i++ {
		builder.WriteString("Rust ownership keeps memory safe without a tracing garbage collector. ")
		builder.WriteString("Borrow checking prevents data races while still allowing zero-cost abstractions. ")
		builder.WriteString("Traits, lifetimes, and generics make reusable APIs explicit. ")
		if i%5 == 0 {
			builder.WriteString("\n")
		}
		if i%17 == 0 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func buildChineseBenchmarkContent() string {
	var builder strings.Builder
	builder.Grow(256 * 1024)

	for i := 0; i < 1600; i++ {
		builder.WriteString("Rust 的所有权系统在编译期追踪值的移动。")
		builder.WriteString("借用检查器会限制可变引用和共享引用的共存关系。")
		builder.WriteString("生命周期标注让引用的有效范围变得显式。")
		if i%6 == 0 {
			builder.WriteString("\n")
		}
		if i%23 == 0 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func buildMarkdownBenchmarkContent() string {
	var builder strings.Builder
	builder.Grow(256 * 1024)

	for i := 0; i < 420; i++ {
		builder.WriteString("## Rust 模块 ")
		builder.WriteString("ownership\n\n")
		builder.WriteString("- 所有权决定值何时被释放。\n")
		builder.WriteString("- 借用规则决定引用是否可以同时存在。\n")
		builder.WriteString("- trait bound 描述泛型函数需要的能力。\n\n")
		builder.WriteString("When a value moves, the previous binding becomes unavailable. ")
		builder.WriteString("This rule prevents double free without a runtime collector. ")
		builder.WriteString("Pattern matching keeps control flow explicit.\n\n")
	}

	return builder.String()
}

func buildLongSentenceBenchmarkContent() string {
	var builder strings.Builder
	builder.Grow(256 * 1024)

	for i := 0; i < 900; i++ {
		builder.WriteString("This intentionally long sentence talks about ownership borrowing lifetimes async scheduling pinning futures trait objects and zero cost abstractions without introducing many early separator boundaries so the splitter has to handle oversized units gracefully. ")
		if i%3 == 0 {
			builder.WriteString("另一个较长的中文句子用于模拟教程脚本文案中没有频繁短标点的情况从而观察完整句子约束下的分块性能。")
		}
		if i%9 == 0 {
			builder.WriteString("\n\n")
		}
	}

	return builder.String()
}

func buildOneMillionBenchmarkContent() string {
	const targetChars = 1_000_000
	sentences := []string{
		"Rust ownership keeps memory safety explicit. ",
		"Borrow rules prevent aliasing violations in compile time. ",
		"Traits and lifetimes help structure reusable abstractions. ",
		"Pattern matching keeps control flow readable and predictable. ",
		"Async executors move tasks between worker threads with wakeups. ",
		"Can this implementation avoid hidden allocation hot spots? ",
		"Use benchmarks to verify assumptions before refactoring critical paths! ",
		"异步任务在运行时调度中依赖状态机推进。 ",
		"错误处理通常使用Result并保持可组合性。 ",
		"类型系统在边界处提供清晰契约。 ",
		"在并发场景里，Send与Sync约束能帮助减少数据竞争。 ",
		"长上下文切分时要优先保证句子完整性。 ",
		"当分块窗口滑动时，重叠区域也应保持语义边界。 ",
		"这个段落是否会触发更多的分隔符路径？ ",
		"如果候选分隔符不足，递归会退化到更细粒度策略！ ",
		"## Bench Note: recursive splitters should be deterministic. ",
		"- Keep sentence units stable across repeated runs.\n",
		"- Preserve punctuation for downstream position annotation.\n",
		"- Prefer low allocs/op under million-scale corpora.\n",
		"\n\n",
		"\n",
	}
	sentenceChars := make([]int, len(sentences))
	for i, sentence := range sentences {
		sentenceChars[i] = utf8.RuneCountInString(sentence)
	}

	var builder strings.Builder
	builder.Grow(targetChars * 2) // mixed CN/EN, bytes are usually larger than chars

	totalChars := 0
	idx := 0
	for totalChars < targetChars {
		current := idx % len(sentences)
		next := sentences[current]
		builder.WriteString(next)
		totalChars += sentenceChars[current]
		idx++
	}

	return builder.String()
}
