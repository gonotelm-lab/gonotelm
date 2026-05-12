package embedding

import (
	"context"
	"crypto/md5"
	"encoding/hex"

	embedcache "github.com/cloudwego/eino-ext/components/embedding/cache"
)

// embedKeyGenerator avoids leaking raw text into Redis key.
type embedKeyGenerator struct{}

var _ embedcache.Generator = (*embedKeyGenerator)(nil)

func newEmbedKeyGenerator() *embedKeyGenerator {
	return &embedKeyGenerator{}
}

func (*embedKeyGenerator) Generate(
	_ context.Context,
	text string,
	opt embedcache.GeneratorOption,
) string {
	sum := md5.Sum([]byte(text + "-" + opt.Model))
	return hex.EncodeToString(sum[:])
}
