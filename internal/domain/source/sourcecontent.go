package source

import (
	"net/url"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type SourceContent interface {
	Kind() SourceKind
	Bytes() []byte
}

type TextSourceContent struct {
	Text string `json:"text"`
}

var _ SourceContent = (*TextSourceContent)(nil)

func (t *TextSourceContent) Kind() SourceKind {
	return SourceKindText
}

func (t *TextSourceContent) Bytes() []byte {
	b, _ := sonic.Marshal(t)
	return b
}

type UrlSourceContent struct {
	Url string `json:"url"`
}

var _ SourceContent = (*UrlSourceContent)(nil)

func (u *UrlSourceContent) Kind() SourceKind {
	return SourceKindUrl
}

func (u *UrlSourceContent) Bytes() []byte {
	b, _ := sonic.Marshal(u)
	return b
}

type FileSourceContent struct {
	StoreKey string `json:"store_key,omitempty"`
	Filename string `json:"filename,omitempty"`
	Md5      string `json:"md5,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Format   string `json:"format,omitempty"`

	Url string `json:"-"` // output usage
}

var _ SourceContent = (*FileSourceContent)(nil)

func (f *FileSourceContent) Kind() SourceKind {
	return SourceKindFile
}

func (f *FileSourceContent) Bytes() []byte {
	b, _ := sonic.Marshal(f)
	return b
}

func NewSourceContent(k SourceKind, b []byte) (SourceContent, error) {
	switch k {
	case SourceKindText:
		var tc TextSourceContent
		if err := sonic.Unmarshal(b, &tc); err != nil {
			return nil, err
		}
		return &tc, nil
	case SourceKindUrl:
		var uc UrlSourceContent
		if err := sonic.Unmarshal(b, &uc); err != nil {
			return nil, err
		}
		return &uc, nil
	case SourceKindFile:
		var fc FileSourceContent
		if err := sonic.Unmarshal(b, &fc); err != nil {
			return nil, err
		}
		return &fc, nil
	}

	return nil, errors.ErrParams.Msgf("unsupported source kind: %s", k)
}

type ContentIntegrate struct {
	Kind SourceKind
	Text string
	Url  *url.URL
}

func (c *ContentIntegrate) toSourceContent() (SourceContent, error) {
	switch c.Kind {
	case SourceKindText:
		return &TextSourceContent{Text: c.Text}, nil
	case SourceKindUrl:
		return &UrlSourceContent{Url: c.Url.String()}, nil
	case SourceKindFile:
		return &FileSourceContent{}, nil // 文件来源的数据是分两步处理的 第一步暂时不会填入内容
	}
	return nil, errors.ErrParams.Msgf("unsupported source kind: %s", c.Kind)
}
