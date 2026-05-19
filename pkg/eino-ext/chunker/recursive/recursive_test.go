package recursive

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/token"

	einorecursive "github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

func TestRecursiveChunker_SlidingWindowKeepsCompleteSentences(t *testing.T) {
	transformer, err := NewSplitter(context.Background(), &Config{
		ChunkSize:   16,
		OverlapSize: 7,
		Separators:  []string{". "},
		KeepType:    KeepTypeEnd,
		LenFunc:     func(s string) int { return len(s) },
	})
	if err != nil {
		t.Fatal(err)
	}

	docs, err := transformer.Transform(context.Background(), []*schema.Document{
		{ID: "doc", Content: "A1. B222. C333. D444. E555."},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := contentsOf(docs)
	want := []string{
		"A1. B222. C333. ",
		"C333. D444. ",
		"D444. E555.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("chunks mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestRecursiveChunker_RecursivelySplitsLargeUnits(t *testing.T) {
	transformer, err := NewSplitter(context.Background(), &Config{
		ChunkSize:   25,
		OverlapSize: 0,
		Separators:  []string{"\n\n", ". "},
		KeepType:    KeepTypeEnd,
		LenFunc:     func(s string) int { return len(s) },
	})
	if err != nil {
		t.Fatal(err)
	}

	docs, err := transformer.Transform(context.Background(), []*schema.Document{
		{ID: "doc", Content: "Intro line\n\nRust owns. Borrow safely. Move values."},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := contentsOf(docs)
	want := []string{
		"Intro line\n\nRust owns. ",
		"Borrow safely. ",
		"Move values.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("chunks mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestRecursiveChunker_DoesNotCutSingleOversizedSentence(t *testing.T) {
	transformer, err := NewSplitter(context.Background(), &Config{
		ChunkSize:   5,
		OverlapSize: 2,
		Separators:  []string{". "},
		KeepType:    KeepTypeEnd,
		LenFunc:     func(s string) int { return len(s) },
	})
	if err != nil {
		t.Fatal(err)
	}

	docs, err := transformer.Transform(context.Background(), []*schema.Document{
		{ID: "doc", Content: "single sentence without separator"},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := contentsOf(docs)
	want := []string{"single sentence without separator"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("chunks mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestRecursiveChunker_ImplementsTransformerAndCopiesMetadata(t *testing.T) {
	var _ document.Transformer = (*recursiveChunker)(nil)

	transformer, err := NewSplitter(context.Background(), &Config{
		ChunkSize:   8,
		OverlapSize: 0,
		Separators:  []string{". "},
		KeepType:    KeepTypeEnd,
		IDGenerator: func(ctx context.Context, originalID string, splitIndex int) string {
			return originalID + "_chunk"
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	sourceMeta := map[string]any{"kind": "rust"}
	docs, err := transformer.Transform(context.Background(), []*schema.Document{
		{ID: "doc", Content: "A. B. C.", MetaData: sourceMeta},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(docs) == 0 {
		t.Fatal("expected chunks")
	}
	if docs[0].ID != "doc_chunk" {
		t.Fatalf("unexpected id: %s", docs[0].ID)
	}
	if docs[0].MetaData["kind"] != "rust" {
		t.Fatalf("metadata was not copied: %#v", docs[0].MetaData)
	}
	docs[0].MetaData["kind"] = "changed"
	if sourceMeta["kind"] != "rust" {
		t.Fatalf("source metadata should not be modified")
	}
}

func contentsOf(docs []*schema.Document) []string {
	ret := make([]string, 0, len(docs))
	for _, doc := range docs {
		ret = append(ret, doc.Content)
	}
	return ret
}

func TestChunkTransformer_Transform_Recusrive(t *testing.T) {
	chunkSize := 500
	overlapSize := 75
	rt, _ := einorecursive.NewSplitter(t.Context(),
		&einorecursive.Config{
			ChunkSize:   chunkSize,
			OverlapSize: overlapSize,
			LenFunc:     token.EstimateToken,
			Separators:  []string{"\n\n", "\n", ".", ",", " ", "", "?", "!", "，", "。", "？", "！"},
		})

	text, _ := os.ReadFile("../testdata/test1.txt")
	content := string(text)
	docs, err := rt.Transform(t.Context(), []*schema.Document{
		{
			ID:      "test1",
			Content: content,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	for idx, doc := range docs {
		fmt.Printf("===================[chunk %d]===================\n", idx)
		fmt.Println(doc.Content)
	}
}

func TestChunkTransformer_Transform_Recusrive_New(t *testing.T) {
	chunkSize := 500
	overlapSize := 75
	rt, _ := NewSplitter(t.Context(), &Config{
		ChunkSize:   chunkSize,
		OverlapSize: overlapSize,
		LenFunc:     token.EstimateToken,
		KeepType:    KeepTypeEnd,
		Separators:  []string{"\n\n", "\n", ".", " ", "", "?", "!", "。", "？", "！"},
	})

	text, _ := os.ReadFile("../testdata/test1.txt")
	content := string(text)
	docs, err := rt.Transform(t.Context(), []*schema.Document{
		{
			ID:      "test1",
			Content: content,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	for idx, doc := range docs {
		fmt.Printf("===================[chunk %d]===================\n", idx)
		fmt.Println(doc.Content)
	}
}
