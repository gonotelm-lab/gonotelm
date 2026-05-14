package transformer

import (
	"github.com/cloudwego/eino/components/document"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
)

const (
	ChunkHtmlSplitMethod      = "html"
	ChunkMarkdownSplitMethod  = "markdown"
	ChunkRecursiveSplitMethod = "recursive"
)

type chunkTransformOption struct {
	splitMethod string
}

func defaultChunkTransformOption() *chunkTransformOption {
	return &chunkTransformOption{
		splitMethod: ChunkRecursiveSplitMethod,
	}
}

func WithChunkSplitMethod(method string) document.TransformerOption {
	return document.WrapTransformerImplSpecificOptFn(func(opt *chunkTransformOption) {
		opt.splitMethod = normalizeChunkSplitMethod(method)
	})
}

func WithChunkSplitMethodByMime(mimeType string) document.TransformerOption {
	return WithChunkSplitMethod(ResolveChunkSplitMethodByMime(mimeType))
}

func GetChunkSplitMethodOption(opts ...document.TransformerOption) string {
	customOpts := defaultChunkTransformOption()
	document.GetTransformerImplSpecificOptions(customOpts, opts...)
	return normalizeChunkSplitMethod(customOpts.splitMethod)
}

func ResolveChunkSplitMethodByMime(mimeType string) string {
	switch mimeType {
	case model.MimeTypePDF, model.MimeTypeWord, model.MimeTypeEPUB:
		return ChunkMarkdownSplitMethod
	default:
		return ChunkRecursiveSplitMethod
	}
}

func normalizeChunkSplitMethod(method string) string {
	switch method {
	case ChunkHtmlSplitMethod, ChunkMarkdownSplitMethod, ChunkRecursiveSplitMethod:
		return method
	default:
		return ChunkRecursiveSplitMethod
	}
}
