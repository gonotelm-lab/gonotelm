package string

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/token"
)

func TestMergeChunksEmptyInput(t *testing.T) {
	if got := MergeChunks(nil, 10); got != nil {
		t.Fatalf("expect nil for nil input, got %v", got)
	}

	if got := MergeChunks([]string{}, 10); got != nil {
		t.Fatalf("expect nil for empty input, got %v", got)
	}
}

func TestMergeChunksNonPositiveTarget(t *testing.T) {
	chunks := []string{"a", "b", "c"}
	got := MergeChunks(chunks, 0)
	if !reflect.DeepEqual(got, chunks) {
		t.Fatalf("expect %v, got %v", chunks, got)
	}
}

func TestMergeChunksByApproxTokenLength(t *testing.T) {
	chunks := []string{"a", "b", "c", "d", "e"}
	got := MergeChunks(chunks, 2)
	want := []string{"ab", "cd", "e"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expect %v, got %v", want, got)
	}
}

func TestMergeChunksKeepOversizedChunk(t *testing.T) {
	largeChunk := strings.Repeat("Rust ownership and borrowing. ", 120)
	chunks := []string{"a", largeChunk, "b"}

	got := MergeChunks(chunks, token.EstimateToken("a")+1)
	want := []string{"a", largeChunk, "b"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expect %v, got %v", want, got)
	}
}

func TestMergeChunksLargeText(t *testing.T) {
	const chunkCount = 2500
	unit := strings.Repeat("Rust 所有权与生命周期 ownership and lifetime. ", 24)
	approx := token.EstimateToken(unit) * 8
	if approx <= 0 {
		approx = 1
	}

	chunks := make([]string, 0, chunkCount)
	var original strings.Builder
	original.Grow((len(unit) + 2) * chunkCount)

	for i := 0; i < chunkCount; i++ {
		chunk := unit + strconv.Itoa(i%10)
		chunks = append(chunks, chunk)
		original.WriteString(chunk)
	}

	got := MergeChunks(chunks, approx)
	if len(got) == 0 {
		t.Fatalf("expect non-empty result")
	}

	if len(got) >= len(chunks) {
		t.Fatalf("expect merged chunk count less than source, source=%d merged=%d", len(chunks), len(got))
	}

	mergedText := strings.Join(got, "")
	if mergedText != original.String() {
		t.Fatalf("merged text mismatch")
	}
}
