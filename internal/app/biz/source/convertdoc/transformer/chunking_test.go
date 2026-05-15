package transformer

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

func TestChunkTransformer_Transform_AssignsPositions_Recursive(t *testing.T) {
	Convey("ChunkTransformer recursive split should annotate byte and rune positions", t, func() {
		transformer := NewChunkTransformer(90, 24, func(s string) int { return len(s) })
		source := strings.Join([]string{
			"Rust ownership keeps memory safe without runtime GC.",
			strings.Repeat("borrow checker enforces aliasing rules. ", 10),
			strings.Repeat("traits and lifetimes improve code reuse. ", 8),
		}, "\n")

		chunks, err := transformer.Transform(
			context.Background(),
			[]*schema.Document{
				{
					ID:      "recursive_source",
					Content: source,
				},
			},
			WithChunkSplitMethod(ChunkRecursiveSplitMethod),
		)

		So(err, ShouldBeNil)
		So(len(chunks), ShouldBeGreaterThan, 2)
		So(assertChunkPositionsMatchSource(source, chunks), ShouldBeNil)

		hasOverlap, err := hasOverlappedChunkPair(chunks)
		So(err, ShouldBeNil)
		So(hasOverlap, ShouldBeTrue)
	})
}

func TestChunkTransformer_Transform_AssignsPositions_MarkdownWithRecursiveFallback(t *testing.T) {
	Convey("ChunkTransformer markdown split with recursive fallback should annotate positions", t, func() {
		transformer := NewChunkTransformer(120, 24, func(s string) int { return len(s) })
		source := strings.Join([]string{
			"# Rust 并发模型",
			"Rust 通过所有权与借用规则限制数据竞争。",
			"",
			"## Send 与 Sync",
			strings.Repeat("Send 和 Sync 的边界由类型系统静态保证。", 22),
			"",
			"## Tokio 调度",
			strings.Repeat("异步任务调度依赖 Future、Waker 与 Pin 语义。", 20),
		}, "\n")

		chunks, err := transformer.Transform(
			context.Background(),
			[]*schema.Document{
				{
					ID:      "markdown_source",
					Content: source,
					MetaData: map[string]any{
						"kind": "markdown",
					},
				},
			},
			WithChunkSplitMethod(ChunkMarkdownSplitMethod),
		)

		So(err, ShouldBeNil)
		So(len(chunks), ShouldBeGreaterThan, 5)
		So(assertChunkPositionsMatchSource(source, chunks), ShouldBeNil)
	})
}

func TestAnnotateChunkPositions_DuplicateChunksCanBeDistinguished(t *testing.T) {
	Convey("annotateChunkPositions should distinguish duplicate chunks", t, func() {
		source := "重复段落A|重复段落A|重复段落A"
		docs := []*schema.Document{
			{ID: "dup_1", Content: "重复段落A"},
			{ID: "dup_2", Content: "重复段落A"},
			{ID: "dup_3", Content: "重复段落A"},
		}
		wantRune := [][2]int{
			{0, 5},
			{6, 11},
			{12, 17},
		}
		wantByte := [][2]int{
			{0, 13},
			{14, 27},
			{28, 41},
		}

		annotateChunkPositions(source, docs)
		So(assertDuplicateChunkPositions(source, docs, wantRune, wantByte), ShouldBeNil)
	})
}

func assertChunkPositionsMatchSource(source string, chunks []*schema.Document) error {
	sourceRunes := []rune(source)
	prevRuneStart := -1
	prevRuneEnd := -1
	prevByteStart := -1
	prevByteEnd := -1

	for idx, chunk := range chunks {
		pos, err := mustChunkPosition(chunk)
		if err != nil {
			return fmt.Errorf("chunk[%d] invalid metadata: %w", idx, err)
		}
		if pos.RuneStart < 0 || pos.RuneEnd > len(sourceRunes) || pos.RuneStart >= pos.RuneEnd {
			return fmt.Errorf("chunk[%d] has invalid rune range: start=%d end=%d", idx, pos.RuneStart, pos.RuneEnd)
		}
		if pos.ByteStart < 0 || pos.ByteEnd > len(source) || pos.ByteStart >= pos.ByteEnd {
			return fmt.Errorf("chunk[%d] has invalid byte range: start=%d end=%d", idx, pos.ByteStart, pos.ByteEnd)
		}

		if got := string(sourceRunes[pos.RuneStart:pos.RuneEnd]); got != chunk.Content {
			return fmt.Errorf("chunk[%d] content mismatch by rune position: got=%q want=%q", idx, got, chunk.Content)
		}
		if got := source[pos.ByteStart:pos.ByteEnd]; got != chunk.Content {
			return fmt.Errorf("chunk[%d] content mismatch by byte position: got=%q want=%q", idx, got, chunk.Content)
		}

		if prevRuneStart >= 0 && pos.RuneStart < prevRuneStart {
			return fmt.Errorf("chunk[%d] rune start not monotonic: prev=%d curr=%d", idx, prevRuneStart, pos.RuneStart)
		}
		if prevRuneEnd >= 0 && pos.RuneEnd <= prevRuneEnd {
			return fmt.Errorf("chunk[%d] rune end should keep moving forward: prev=%d curr=%d", idx, prevRuneEnd, pos.RuneEnd)
		}
		if prevByteStart >= 0 && pos.ByteStart < prevByteStart {
			return fmt.Errorf("chunk[%d] byte start not monotonic: prev=%d curr=%d", idx, prevByteStart, pos.ByteStart)
		}
		if prevByteEnd >= 0 && pos.ByteEnd <= prevByteEnd {
			return fmt.Errorf("chunk[%d] byte end should keep moving forward: prev=%d curr=%d", idx, prevByteEnd, pos.ByteEnd)
		}

		prevRuneStart = pos.RuneStart
		prevRuneEnd = pos.RuneEnd
		prevByteStart = pos.ByteStart
		prevByteEnd = pos.ByteEnd
	}

	return nil
}

type chunkPosition struct {
	RuneStart int
	RuneEnd   int
	ByteStart int
	ByteEnd   int
}

func hasOverlappedChunkPair(chunks []*schema.Document) (bool, error) {
	prevRuneEnd := -1
	for _, chunk := range chunks {
		pos, err := mustChunkPosition(chunk)
		if err != nil {
			return false, err
		}
		if prevRuneEnd >= 0 && pos.RuneStart < prevRuneEnd {
			return true, nil
		}
		prevRuneEnd = pos.RuneEnd
	}

	return false, nil
}

func mustChunkPosition(chunk *schema.Document) (chunkPosition, error) {
	if chunk == nil {
		return chunkPosition{}, fmt.Errorf("chunk is nil")
	}
	if chunk.MetaData == nil {
		return chunkPosition{}, fmt.Errorf("chunk metadata is nil, content=%q", chunk.Content)
	}

	startRaw, ok := chunk.MetaData[ChunkMetaPositionStartKey]
	if !ok {
		return chunkPosition{}, fmt.Errorf("chunk missing %s metadata, content=%q", ChunkMetaPositionStartKey, chunk.Content)
	}
	endRaw, ok := chunk.MetaData[ChunkMetaPositionEndKey]
	if !ok {
		return chunkPosition{}, fmt.Errorf("chunk missing %s metadata, content=%q", ChunkMetaPositionEndKey, chunk.Content)
	}
	byteStartRaw, ok := chunk.MetaData[ChunkMetaPositionByteStartKey]
	if !ok {
		return chunkPosition{}, fmt.Errorf("chunk missing %s metadata, content=%q", ChunkMetaPositionByteStartKey, chunk.Content)
	}
	byteEndRaw, ok := chunk.MetaData[ChunkMetaPositionByteEndKey]
	if !ok {
		return chunkPosition{}, fmt.Errorf("chunk missing %s metadata, content=%q", ChunkMetaPositionByteEndKey, chunk.Content)
	}

	runeStart, ok := startRaw.(int)
	if !ok {
		return chunkPosition{}, fmt.Errorf("chunk start metadata type invalid: %T", startRaw)
	}
	runeEnd, ok := endRaw.(int)
	if !ok {
		return chunkPosition{}, fmt.Errorf("chunk end metadata type invalid: %T", endRaw)
	}
	byteStart, ok := byteStartRaw.(int)
	if !ok {
		return chunkPosition{}, fmt.Errorf("chunk byte start metadata type invalid: %T", byteStartRaw)
	}
	byteEnd, ok := byteEndRaw.(int)
	if !ok {
		return chunkPosition{}, fmt.Errorf("chunk byte end metadata type invalid: %T", byteEndRaw)
	}

	return chunkPosition{
		RuneStart: runeStart,
		RuneEnd:   runeEnd,
		ByteStart: byteStart,
		ByteEnd:   byteEnd,
	}, nil
}

func assertDuplicateChunkPositions(
	source string,
	docs []*schema.Document,
	wantRune [][2]int,
	wantByte [][2]int,
) error {
	if len(docs) != len(wantRune) || len(docs) != len(wantByte) {
		return fmt.Errorf(
			"input length mismatch: docs=%d wantRune=%d wantByte=%d",
			len(docs), len(wantRune), len(wantByte),
		)
	}

	sourceRunes := []rune(source)
	for idx, doc := range docs {
		pos, err := mustChunkPosition(doc)
		if err != nil {
			return fmt.Errorf("duplicate chunk[%d] invalid metadata: %w", idx, err)
		}

		if pos.RuneStart != wantRune[idx][0] || pos.RuneEnd != wantRune[idx][1] {
			return fmt.Errorf(
				"duplicate chunk[%d] rune range mismatch: got=(%d,%d) want=(%d,%d)",
				idx, pos.RuneStart, pos.RuneEnd, wantRune[idx][0], wantRune[idx][1],
			)
		}
		if pos.ByteStart != wantByte[idx][0] || pos.ByteEnd != wantByte[idx][1] {
			return fmt.Errorf(
				"duplicate chunk[%d] byte range mismatch: got=(%d,%d) want=(%d,%d)",
				idx, pos.ByteStart, pos.ByteEnd, wantByte[idx][0], wantByte[idx][1],
			)
		}
		if got := string(sourceRunes[pos.RuneStart:pos.RuneEnd]); got != doc.Content {
			return fmt.Errorf("duplicate chunk[%d] content mismatch: got=%q want=%q", idx, got, doc.Content)
		}
		if got := source[pos.ByteStart:pos.ByteEnd]; got != doc.Content {
			return fmt.Errorf("duplicate chunk[%d] byte content mismatch: got=%q want=%q", idx, got, doc.Content)
		}
	}

	return nil
}
