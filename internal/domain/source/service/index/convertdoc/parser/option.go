package parser

import (
	"mime"

	einoparser "github.com/cloudwego/eino/components/document/parser"
)

type customParseOption struct {
	fileMime string // 文件来源时的mime type
	fileExt  string // 文件来源时的format
}

func WithFileMime(mimeType string) einoparser.Option {
	return einoparser.WrapImplSpecificOptFn(func(t *customParseOption) {
		t.fileMime = mimeType
	})
}

func WithFileExt(ext string) einoparser.Option {
	return einoparser.WrapImplSpecificOptFn(func(t *customParseOption) {
		t.fileExt = ext
	})
}

func ResolveSourceMime(sourceMime, ext string) string {
	if sourceMime != "" {
		return sourceMime
	}

	mediaType := mime.TypeByExtension(ext)
	parsedMime, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		return ""
	}

	return parsedMime
}
