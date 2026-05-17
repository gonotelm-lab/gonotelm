package model

import (
	"fmt"
	"log/slog"

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
	Id            Id           `json:"id"`
	NotebookId    Id           `json:"notebook_id"`
	Kind          SourceKind   `json:"kind"`
	Status        SourceStatus `json:"status"`
	Title         string       `json:"title"`
	Content       []byte       `json:"content"`
	ParsedContent []byte       `json:"parsed_content,omitempty"`
	Abstract      string       `json:"abstract"`
	OwnerId       string       `json:"owner_id"`
	UpdatedAt     int64        `json:"updated_at"`
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

func NewSourceFrom(s *schema.Source) *Source {
	source := &Source{
		Id:            s.Id,
		NotebookId:    s.NotebookId,
		Kind:          SourceKind(s.Kind),
		Status:        SourceStatus(s.Status),
		Title:         s.Title,
		Content:       s.Content,
		ParsedContent: s.ParsedContent,
		Abstract:      s.Abstract,
		OwnerId:       s.OwnerId,
		UpdatedAt:     s.UpdatedAt,
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
	MimeTypeText     = "text/plain"
	MimeTypeMarkdown = "text/markdown"
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

type DecodedSource struct {
	*Source

	ContentText *TextSourceContent `json:"content_text,omitempty"`
	ContentUrl  *UrlSourceContent  `json:"content_url,omitempty"`
	ContentFile *FileSourceContent `json:"content_file,omitempty"`

	ParsedContent *ParsedSourceContent `json:"parsed_content,omitempty"`
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

	if s.ParsedContent != nil {
		if err := sonic.Unmarshal(s.ParsedContent, &sc.ParsedContent); err != nil {
			// log only
			slog.Error("unmarshal parsed content failed", "source_id", s.Id, "err", err)
		}
	}

	return &sc, nil
}

type ParsedSourceContent struct {
	StoreKey string `json:"store_key,omitempty"`
}
