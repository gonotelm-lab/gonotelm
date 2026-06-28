package source

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const (
	MaxSourceTitleLength   = 255
	MaxOwnerIdLength       = 255
	MaxUploadFileSizeBytes = 100 * 1024 * 1024 // 100MB
)

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

func (s SourceStatus) IsInited() bool {
	return s == SourceStatusInited
}

func (s SourceStatus) IsUploading() bool {
	return s == SourceStatusUploading
}

func (s SourceStatus) IsPreparing() bool {
	return s == SourceStatusPreparing
}

func (s SourceStatus) IsReady() bool {
	return s == SourceStatusReady
}

func (s SourceStatus) IsFailed() bool {
	return s == SourceStatusFailed
}

type Source struct {
	entity.Base

	NotebookId       valobj.Id
	Kind             SourceKind
	Status           SourceStatus
	Title            string
	Abstract         string
	OwnerId          string
	Content          SourceContent
	ParsedContentKey string
}

func NewSource(
	notebookId valobj.Id,
	kind SourceKind,
	ownerId string,
	content *ContentIntegrate,
) (*Source, error) {
	s := &Source{
		Base:       entity.NewBase(),
		NotebookId: notebookId,
		Kind:       kind,
		Status:     SourceStatusInited,
		OwnerId:    ownerId,
	}
	sourceContent, err := content.toSourceContent()
	if err != nil {
		return nil, err
	}
	s.Content = sourceContent

	if err := s.validate(); err != nil {
		return nil, err
	}

	if !s.Kind.IsFile() {
		s.addPreparationEvent(false)
	}

	return s, nil
}

func (s *Source) validate() error {
	if len := utf8.RuneCountInString(s.OwnerId); len > MaxOwnerIdLength {
		return errors.ErrParams.Msgf("owner id is too long, length=%d", len)
	}

	if s.NotebookId.IsZero() {
		return errors.ErrParams.Msgf("notebook id is required")
	}

	if !s.Kind.Supported() {
		return errors.ErrParams.Msgf("unsupported source kind: %s", s.Kind)
	}

	if s.Content == nil {
		return errors.ErrParams.Msgf("content is required")
	}

	if s.Content.Kind() != s.Kind {
		return errors.ErrParams.Msgf("content kind mismatch, content_kind=%s, source_kind=%s", s.Content.Kind(), s.Kind)
	}

	return nil
}

func (s *Source) addPreparationEvent(isRetry bool) {
	s.Base.AddEvent(
		NewPreparationEvent(
			s.Id,
			s.NotebookId,
			s.Kind,
			s.Status,
			s.OwnerId,
			isRetry,
		),
	)
}

type UploadFileParams struct {
	Filename string
	MimeType string
	Size     int64
	Md5      string
}

func (p *UploadFileParams) validate() error {
	if p.Size > MaxUploadFileSizeBytes {
		return errors.ErrParams.Msgf("file size is too large, size=%d", p.Size)
	}

	if !SupportedFileMimeType(p.MimeType) {
		return errors.ErrParams.Msgf("unsupported mime type: %s", p.MimeType)
	}

	return nil
}

func (s *Source) UploadFile(ctx context.Context, params *UploadFileParams) error {
	if err := s.checkUploadable(); err != nil {
		return err
	}

	if err := params.validate(); err != nil {
		return errors.WithMessage(err, "validate upload file params failed")
	}

	storeKey := s.formatFileStoreKey(params)

	fileContent := &FileSourceContent{
		StoreKey: storeKey,
		Filename: params.Filename,
		Size:     params.Size,
		Md5:      params.Md5,
		Format:   params.MimeType,
	}
	s.Content = fileContent

	return nil
}

func (s *Source) checkUploadable() error {
	if !s.Kind.IsFile() {
		return errors.ErrParams.Msgf("source is not a file, kind=%s", s.Kind)
	}
	if !s.Status.IsInited() {
		return errors.ErrParams.Msgf("source is not inited, status=%s", s.Status)
	}

	return nil
}

func (s *Source) formatFileStoreKey(params *UploadFileParams) string {
	var (
		notebookId = s.NotebookId.String()
		sourceId   = s.Id.String()
		// take extension from input filename
		ext = filepath.Ext(params.Filename)
	)

	return fmt.Sprintf("file/%s/%s%s", notebookId, sourceId, ext)
}

func (s *Source) GetFileContent() (*FileSourceContent, error) {
	if s.Content == nil {
		return nil, errors.ErrParams.Msgf("source content is nil")
	}

	if s.Content.Kind() != SourceKindFile {
		return nil, errors.ErrParams.Msgf("source content is not a file, kind=%s", s.Content.Kind())
	}

	fileContent, ok := s.Content.(*FileSourceContent)
	if !ok {
		return nil, errors.ErrParams.Msgf("source content is not a file, kind=%s", s.Content.Kind())
	}

	return fileContent, nil
}

func (s *Source) GetTextContent() (*TextSourceContent, error) {
	if s.Content == nil {
		return nil, errors.ErrParams.Msgf("source content is nil")
	}

	if s.Content.Kind() != SourceKindText {
		return nil, errors.ErrParams.Msgf("source content is not a text, kind=%s", s.Content.Kind())
	}

	textContent, ok := s.Content.(*TextSourceContent)
	if !ok {
		return nil, errors.ErrParams.Msgf("source content is not a text, kind=%s", s.Content.Kind())
	}

	return textContent, nil
}

func (s *Source) MarkPreparing() {
	s.Status = SourceStatusPreparing
	s.addPreparationEvent(false)
}

func (s *Source) MarkFailed() {
	s.Status = SourceStatusFailed
}

func (s *Source) RetryPreparation() error {
	if !s.Status.IsFailed() {
		return errors.ErrParams.Msg("no need to retry")
	}

	s.Status = SourceStatusPreparing
	s.addPreparationEvent(true)
	return nil
}

func (s *Source) UpdateTitle(title string) error {
	nextTitle := strings.TrimSpace(title)
	if nextTitle == "" {
		return errors.ErrParams.Msg("source title is empty")
	}
	if titleLen := utf8.RuneCountInString(nextTitle); titleLen > MaxSourceTitleLength {
		return errors.ErrParams.Msgf("source title is too long, length=%d", titleLen)
	}
	if s.Title == nextTitle {
		return nil
	}

	s.Title = nextTitle
	return nil
}

func (s *Source) Delete() {
	s.Base.Delete()
	s.addDeleteEvent()
}

func (s *Source) addDeleteEvent() {
	s.Base.AddEvent(
		NewDeleteEvent(
			s.Id,
			s.NotebookId,
		),
	)
}