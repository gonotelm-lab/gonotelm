package util

import "testing"

func TestLocateChunkStart(t *testing.T) {
	t.Parallel()

	t.Run("first chunk by direct index", func(t *testing.T) {
		start, ok := LocateChunkStart("abc\ndef", "abc", "", 0, 0, false)
		if !ok || start != 0 {
			t.Fatalf("LocateChunkStart() = (%d, %v), want (0, true)", start, ok)
		}
	})

	t.Run("next chunk with overlap", func(t *testing.T) {
		source := "hello-world-end"
		prevChunk := "hello-world"
		chunk := "world-end"
		start, ok := LocateChunkStart(source, chunk, prevChunk, 0, len(prevChunk), true)
		if !ok {
			t.Fatalf("LocateChunkStart() overlap not found")
		}
		if got := source[start : start+len(chunk)]; got != chunk {
			t.Fatalf("LocateChunkStart() got %q, want %q", got, chunk)
		}
	})

	t.Run("skip duplicated previous range", func(t *testing.T) {
		source := "foo foo foo"
		prevChunk := "foo foo"
		chunk := "foo"
		prevStart := 0
		prevEnd := len(prevChunk)
		start, ok := LocateChunkStart(source, chunk, prevChunk, prevStart, prevEnd, true)
		if !ok {
			t.Fatalf("LocateChunkStart() duplicate scan not found")
		}
		if start < prevStart {
			t.Fatalf("LocateChunkStart() start=%d should not move backwards", start)
		}
	})
}

func TestBuildChunkByteSpans(t *testing.T) {
	t.Parallel()

	source := "hello world\nhello golang\n"
	chunks := []string{"hello world\n", "world\nhello ", "hello golang\n"}
	spans := BuildChunkByteSpans(source, chunks)
	if len(spans) != len(chunks) {
		t.Fatalf("BuildChunkByteSpans() len=%d, want=%d", len(spans), len(chunks))
	}
	for idx, span := range spans {
		if span.StartByte < 0 || span.EndByte <= span.StartByte {
			t.Fatalf("span[%d] invalid: %+v", idx, span)
		}
		got := source[span.StartByte:span.EndByte]
		if got != chunks[idx] {
			t.Fatalf("span[%d] content=%q, want=%q", idx, got, chunks[idx])
		}
	}
}

func TestRuneByteOffsetConversion(t *testing.T) {
	t.Parallel()

	text := "A中B😀C"
	index := BuildRuneIndexByByteOffset(text)

	if got := ByteOffsetToRuneOffset(index, 0); got != 0 {
		t.Fatalf("ByteOffsetToRuneOffset(0)=%d, want 0", got)
	}

	bytePosOfB := len("A中")
	if got := ByteOffsetToRuneOffset(index, bytePosOfB); got != 2 {
		t.Fatalf("ByteOffsetToRuneOffset(bytePosOfB)=%d, want 2", got)
	}

	if got := ByteOffsetToRuneOffset(index, len(text)); got != 5 {
		t.Fatalf("ByteOffsetToRuneOffset(end)=%d, want 5", got)
	}
}
