package embedding

import (
	"context"
	"strings"
	"testing"

	embedcache "github.com/cloudwego/eino-ext/components/embedding/cache"
)

func TestEmbedKeyGeneratorGenerate(t *testing.T) {
	gen := newEmbedKeyGenerator()

	text := "Four days after the indictment was made public"
	opt := embedcache.GeneratorOption{Model: "mxbai-embed-large"}

	key := gen.Generate(context.Background(), text, opt)
	if len(key) != 32 {
		t.Fatalf("unexpected key length: got=%d want=32", len(key))
	}
	if strings.Contains(key, text) {
		t.Fatalf("key should not contain raw text")
	}

	sameKey := gen.Generate(context.Background(), text, opt)
	if key != sameKey {
		t.Fatalf("key should be deterministic: got=%q want=%q", sameKey, key)
	}

	anotherModelKey := gen.Generate(
		context.Background(),
		text,
		embedcache.GeneratorOption{Model: "text-embedding-3-small"},
	)
	if anotherModelKey == key {
		t.Fatalf("key should differ when model differs")
	}
}
