package entity

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	coreentity "github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
	sourceevent "github.com/gonotelm-lab/gonotelm/internal/domain/source/event"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const (
	MaxSourceTitleLength   = 255
	MaxOwnerIdLength       = 255
	MaxUploadFileSizeBytes = 100 * 1024 * 1024 // 100MB

	// 一篇来源最大允许token数量
	MaxSourceTextContentToken = 1_000_000 // 100k
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

type Source struct {
	coreentity.Base

	NotebookId       valobj.Id
	Kind             vo.SourceKind
	Status           vo.SourceStatus
	Title            string
	Abstract         string
	OwnerId          string
	Content          SourceContent
	ParsedContentKey string
}

func NewSource(
	notebookId valobj.Id,
	kind vo.SourceKind,
	ownerId string,
	content *ContentUnion,
) (*Source, error) {
	s := &Source{
		Base:       coreentity.NewBase(),
		NotebookId: notebookId,
		Kind:       kind,
		Status:     vo.SourceStatusInited,
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
	s.UpdateTime = valobj.NewTime()

	return nil
}

func (s *Source) UploadParsedContent() error {
	storeKey := s.formatParsedContentStoreKey()
	s.UpdateTime = valobj.NewTime()
	s.ParsedContentKey = storeKey

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
		ext        = filepath.Ext(params.Filename)
	)

	return fmt.Sprintf("file/%s/%s%s", notebookId, sourceId, ext)
}

func (s *Source) formatParsedContentStoreKey() string {
	const parsedContentStorePrefix = "parsed_file/"
	var (
		notebookId = s.NotebookId.String()
		sourceId   = s.Id.String()
	)

	return parsedContentStorePrefix + notebookId + "/" + sourceId
}

func (s *Source) GetFileContent() (*FileSourceContent, error) {
	if s.Content == nil {
		return nil, errors.ErrParams.Msgf("source content is nil")
	}

	if s.Content.Kind() != vo.SourceKindFile {
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

	if s.Content.Kind() != vo.SourceKindText {
		return nil, errors.ErrParams.Msgf("source content is not a text, kind=%s", s.Content.Kind())
	}

	textContent, ok := s.Content.(*TextSourceContent)
	if !ok {
		return nil, errors.ErrParams.Msgf("source content is not a text, kind=%s", s.Content.Kind())
	}

	return textContent, nil
}

func (s *Source) GetUrlContent() (*UrlSourceContent, error) {
	if s.Content == nil {
		return nil, errors.ErrParams.Msgf("source content is nil")
	}

	if s.Content.Kind() != vo.SourceKindUrl {
		return nil, errors.ErrParams.Msgf("source content is not a url, kind=%s", s.Content.Kind())
	}

	urlContent, ok := s.Content.(*UrlSourceContent)
	if !ok {
		return nil, errors.ErrParams.Msgf("source content is not a url, kind=%s", s.Content.Kind())
	}

	return urlContent, nil
}

func (s *Source) MarkPreparing() {
	s.Status = vo.SourceStatusPreparing
	s.addPreparationEvent(false)
	s.UpdateTime = valobj.NewTime()
}

func (s *Source) MarkFailed() {
	s.Status = vo.SourceStatusFailed
	s.UpdateTime = valobj.NewTime()
}

func (s *Source) MarkUploading() {
	s.Status = vo.SourceStatusUploading
	s.UpdateTime = valobj.NewTime()
}

func (s *Source) MarkReady() {
	s.Status = vo.SourceStatusReady
	s.UpdateTime = valobj.NewTime()
	s.addIndexEvent()
}

func (s *Source) RetryPreparation() error {
	if !s.Status.IsFailed() {
		return errors.ErrParams.Msg("no need to retry")
	}

	s.Status = vo.SourceStatusPreparing
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
	s.UpdateTime = valobj.NewTime()
	return nil
}

func (s *Source) UpdateAbstract(abstract string) {
	s.Abstract = abstract
	s.UpdateTime = valobj.NewTime()
}

func (s *Source) Delete() {
	s.Base.Delete()
	s.addDeleteEvent()
	s.UpdateTime = valobj.NewTime()
}

func (s *Source) addDeleteEvent() {
	s.Base.AddEvent(sourceevent.NewDeleteEvent(s.Id, s.NotebookId))
}

func (s *Source) addPreparationEvent(isRetry bool) {
	s.Base.AddEvent(
		sourceevent.NewPreparationEvent(
			s.Id,
			s.NotebookId,
			s.Kind,
			s.Status,
			s.OwnerId,
			isRetry,
		),
	)
}

func (s *Source) addIndexEvent() {
	s.Base.AddEvent(sourceevent.NewIndexEvent(s.Id, s.NotebookId))
}
