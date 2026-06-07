package model

import (
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
)

type FromBytes interface {
	From(b []byte) error
}

type SourceKind string

const (
	SourceKindText SourceKind = "text"
	SourceKindUrl  SourceKind = "url"
	SourceKindFile SourceKind = "file"
)

func (s SourceKind) IsFile() bool {
	return s == SourceKindFile
}

func (s SourceKind) IsText() bool {
	return s == SourceKindText
}

func (s SourceKind) IsUrl() bool {
	return s == SourceKindUrl
}

func (s SourceKind) String() string {
	return string(s)
}

type SourceStatus string

func (s SourceStatus) String() string {
	return string(s)
}

const (
	SourceStatusInited    SourceStatus = "inited"
	SourceStatusUploading SourceStatus = "uploading"
	SourceStatusPreparing SourceStatus = "preparing"
	SourceStatusReady     SourceStatus = "ready"
	SourceStatusFailed    SourceStatus = "failed"
)

func (s SourceKind) Supported() bool {
	switch s {
	case SourceKindText, SourceKindUrl, SourceKindFile:
		return true
	}

	return false
}

type Source struct {
	Id         Id           `json:"id"`
	NotebookId Id           `json:"notebook_id"`
	Kind       SourceKind   `json:"kind"`
	Status     SourceStatus `json:"status"`
	Title      string       `json:"title"`

	// content为来源的原始内
	// 按照Kind字段 content有不同的结构
	// Kind=text时 TextSourceContent序列化数据
	// Kind=url时  UrlSourceContent序列化数据
	// Kind=file时 FileSourceContent序列化数据
	Content []byte `json:"content"`

	// 原始文档解析后存储的key
	ParsedContentKey string `json:"parsed_content_key,omitempty"`
	Abstract         string `json:"abstract"`
	OwnerId          string `json:"owner_id"`
	UpdatedAt        int64  `json:"updated_at"`
}

func (s *Source) KindText() bool {
	if s != nil {
		return s.Kind == SourceKindText
	}
	return false
}

func (s *Source) KindUrl() bool {
	if s != nil {
		return s.Kind == SourceKindUrl
	}
	return false
}

func (s *Source) KindFile() bool {
	if s != nil {
		return s.Kind == SourceKindFile
	}
	return false
}

func (s *Source) StatusInited() bool {
	if s != nil {
		return s.Status == SourceStatusInited
	}
	return false
}

func (s *Source) StatusUploading() bool {
	if s != nil {
		return s.Status == SourceStatusUploading
	}
	return false
}

func (s *Source) StatusPreparing() bool {
	if s != nil {
		return s.Status == SourceStatusPreparing
	}
	return false
}

func (s *Source) StatusReady() bool {
	if s != nil {
		return s.Status == SourceStatusReady
	}
	return false
}

func (s *Source) StatusFailed() bool {
	if s != nil {
		return s.Status == SourceStatusFailed
	}
	return false
}

func (s *Source) To() *schema.Source {
	return &schema.Source{
		Id:         s.Id,
		NotebookId: s.NotebookId,
		Kind:       string(s.Kind),
		Status:     s.Status.String(),
		Title:      s.Title,
		Content:    s.Content,
		OwnerId:    s.OwnerId,
		UpdatedAt:  s.UpdatedAt,
	}
}

// 获取内容引用相关的对象键 (所有storeKeys)
func (s *Source) ContentRefKeys() ([]string, error) {
	keys := make([]string, 0, 2)
	ds, err := NewDecodedSource(s)
	if err != nil {
		return keys, err
	}

	if key := ds.ContentRefKey(); key != "" && ds.IsContentRef() {
		keys = append(keys, key)
	}
	if ds.ParsedContentKey != "" {
		keys = append(keys, ds.ParsedContentKey)
	}

	return keys, nil
}

func NewSourceFrom(s *schema.Source) *Source {
	source := &Source{
		Id:               s.Id,
		NotebookId:       s.NotebookId,
		Kind:             SourceKind(s.Kind),
		Status:           SourceStatus(s.Status),
		Title:            s.Title,
		Content:          s.Content,
		ParsedContentKey: s.ParsedContentKey,
		Abstract:         s.Abstract,
		OwnerId:          s.OwnerId,
		UpdatedAt:        s.UpdatedAt,
	}

	return source
}

type TextSourceContent struct {
	Text string `json:"text"`
}

var _ FromBytes = (*TextSourceContent)(nil)

func (t *TextSourceContent) From(b []byte) error {
	return sonic.Unmarshal(b, t)
}

type UrlSourceContent struct {
	Url string `json:"url"`
}

var _ FromBytes = (*UrlSourceContent)(nil)

func (u *UrlSourceContent) From(b []byte) error {
	return sonic.Unmarshal(b, u)
}

type FileSourceContent struct {
	StoreKey string `json:"store_key"`
	Filename string `json:"filename"`
	Md5      string `json:"md5"`
	Size     int64  `json:"size"`
	Format   string `json:"format"`

	Url string `json:"-"` // output usage
}

var _ FromBytes = (*FileSourceContent)(nil)

func (f *FileSourceContent) From(b []byte) error {
	return sonic.Unmarshal(b, f)
}

// Supported source file mime types
const (
	MimeTypePDF      = "application/pdf"
	MimeTypeText     = "text/plain; charset=utf-8"
	MimeTypeMarkdown = "text/markdown; charset=utf-8"
	MimeTypeEPUB     = "application/epub+zip"
	MimeTypeWord     = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
)

func SupportedFileMimeType(mimeType string) bool {
	switch mimeType {
	case MimeTypePDF, MimeTypeText, MimeTypeMarkdown, MimeTypeEPUB, MimeTypeWord:
		return true
	}
	return false
}

// DecodedSource 会将content进行解析
//
// 并且带有Url
type DecodedSource struct {
	*Source

	ContentText *TextSourceContent `json:"content_text,omitempty"`
	ContentUrl  *UrlSourceContent  `json:"content_url,omitempty"`
	ContentFile *FileSourceContent `json:"content_file,omitempty"`
}

func NewDecodedSource(s *Source) (*DecodedSource, error) {
	var (
		err error
		sc  = DecodedSource{Source: s}
	)
	switch s.Kind {
	case SourceKindText:
		tc := TextSourceContent{}
		err = tc.From(s.Content)
		if err == nil {
			sc.ContentText = &tc
		}
	case SourceKindUrl:
		uc := UrlSourceContent{}
		err = uc.From(s.Content)
		if err == nil {
			sc.ContentUrl = &uc
		}
	case SourceKindFile:
		var fc FileSourceContent
		err = fc.From(s.Content)
		if err == nil {
			sc.ContentFile = &fc
		}
	default:
		err = fmt.Errorf("unsupported source kind, source_id=%s", s.Id)
	}
	if err != nil {
		return nil, err
	}

	return &sc, nil
}

// 判断content是inline内容还是需要从对象存储拿
// inline: 原始内容直接可以从content字段解析出来
// ref: 原始内容需要从对象存储拿, content字段只是存储原始内容的store_key
func (d *DecodedSource) IsContentRef() bool {
	return d.Kind.IsFile()
}

// 如果是ref内容获取获取url
func (d *DecodedSource) PopulateContentRef(fn func(storeKey string) (string, error)) error {
	if d.Kind.IsFile() {
		url, err := fn(d.ContentFile.StoreKey)
		if err != nil {
			return err
		}

		if url != "" {
			d.ContentFile.Url = url
		}
	}

	return nil
}

func (d *DecodedSource) ContentRefKey() string {
	switch d.Kind {
	case SourceKindFile:
		return d.ContentFile.StoreKey
	default:
		return ""
	}
}

// 带完整Url的来源建模
type FullSource struct {
	*DecodedSource
	ParsedContentUrl string
}
