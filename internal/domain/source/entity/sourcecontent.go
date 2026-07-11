package entity

import (
	"net/url"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type SourceContent interface {
	Kind() vo.SourceKind
	Bytes() []byte
}

type TextSourceContent struct {
	Text string `json:"text"`
}

var _ SourceContent = (*TextSourceContent)(nil)

func (t *TextSourceContent) Kind() vo.SourceKind {
	return vo.SourceKindText
}

func (t *TextSourceContent) Bytes() []byte {
	b, _ := sonic.Marshal(t)
	return b
}

type UrlSourceContent struct {
	Url string `json:"url"`
}

var _ SourceContent = (*UrlSourceContent)(nil)

func (u *UrlSourceContent) Kind() vo.SourceKind {
	return vo.SourceKindUrl
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
}

var _ SourceContent = (*FileSourceContent)(nil)

func (f *FileSourceContent) Kind() vo.SourceKind {
	return vo.SourceKindFile
}

func (f *FileSourceContent) Bytes() []byte {
	b, _ := sonic.Marshal(f)
	return b
}

func NewSourceContent(k vo.SourceKind, b []byte) (SourceContent, error) {
	switch k {
	case vo.SourceKindText:
		var tc TextSourceContent
		if err := sonic.Unmarshal(b, &tc); err != nil {
			return nil, err
		}
		return &tc, nil
	case vo.SourceKindUrl:
		var uc UrlSourceContent
		if err := sonic.Unmarshal(b, &uc); err != nil {
			return nil, err
		}
		return &uc, nil
	case vo.SourceKindFile:
		var fc FileSourceContent
		if err := sonic.Unmarshal(b, &fc); err != nil {
			return nil, err
		}
		return &fc, nil
	}

	return nil, errors.ErrParams.Msgf("unsupported source kind: %s", k)
}

type ContentUnion struct {
	Kind vo.SourceKind

	// one of the following fields must be set
	Text string
	Url  *url.URL
}

func (c *ContentUnion) toSourceContent() (SourceContent, error) {
	switch c.Kind {
	case vo.SourceKindText:
		return &TextSourceContent{Text: c.Text}, nil
	case vo.SourceKindUrl:
		return &UrlSourceContent{Url: c.Url.String()}, nil
	case vo.SourceKindFile:
		return &FileSourceContent{}, nil
	}
	return nil, errors.ErrParams.Msgf("unsupported source kind: %s", c.Kind)
}
