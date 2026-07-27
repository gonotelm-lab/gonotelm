package schema

import (
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcevo "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type TextSourceContent struct {
	Text string `json:"text"`
}

type UrlSourceContent struct {
	Url string `json:"url"`
}

type FileSourceContent struct {
	Url      string `json:"url"` // full url link
	Filename string `json:"filename"`
	Format   string `json:"format"`
}

type Source struct {
	Id     string                `json:"id"`
	Kind   sourcevo.SourceKind   `json:"kind"`
	Status sourcevo.SourceStatus `json:"status"`
	Title  string                `json:"title"`

	Text *TextSourceContent `json:"text,omitempty"`
	Url  *UrlSourceContent  `json:"url,omitempty"`
	File *FileSourceContent `json:"file,omitempty"`

	ParsedContent *SourceParsedContent `json:"parsed_content,omitempty"`
}

type SourceParsedContent struct {
	Url string `json:"url,omitempty"`
}

func ToSourceFromDomain(
	source *sourceentity.Source,
	fileContentUrl, parsedContentUrl string,
) *Source {
	if source == nil {
		return nil
	}

	s := &Source{
		Id:     source.Id.String(),
		Kind:   source.Kind,
		Status: source.Status,
		Title:  source.Title,
		ParsedContent: &SourceParsedContent{
			Url: parsedContentUrl,
		},
	}

	switch {
	case source.Kind.IsText():
		if textContent, err := source.GetTextContent(); err == nil {
			s.Text = &TextSourceContent{Text: textContent.Text}
		}
	case source.Kind.IsUrl():
		if urlContent, ok := source.Content.(*sourceentity.UrlSourceContent); ok {
			s.Url = &UrlSourceContent{Url: urlContent.Url}
		}
	case source.Kind.IsFile():
		if fileContent, err := source.GetFileContent(); err == nil {
			s.File = &FileSourceContent{
				Filename: fileContent.Filename,
				Format:   fileContent.Format,
				Url:      fileContentUrl,
			}
		}
	}

	return s
}

func ToSourceFromDomainDetail(
	detail *sourceentity.SourceDetail,
) *Source {
	return ToSourceFromDomain(
		detail.Source,
		detail.Access.FileContentUrl,
		detail.Access.ParsedContentUrl,
	)
}

func ToSourcesFromDomainDetails(
	details []*sourceentity.SourceDetail,
) []*Source {
	resp := make([]*Source, 0, len(details))
	for _, detail := range details {
		resp = append(resp, ToSourceFromDomainDetail(detail))
	}
	return resp
}

type CreateSourceRequest struct {
	NotebookId uuid.UUID `json:"notebook_id,required"`
	Kind       string    `json:"kind,required"`

	Text string `json:"text"`
	Url  string `json:"url"`

	// internal use
	parsedUrl *url.URL
}

func (r *CreateSourceRequest) ParsedURL() *url.URL {
	return r.parsedUrl
}

func (r *CreateSourceRequest) Validate() error {
	mk := sourcevo.SourceKind(r.Kind)
	if !mk.Supported() {
		return errors.Errorf("invalid source kind: %s", r.Kind)
	}

	switch mk {
	case sourcevo.SourceKindText:
		if r.Text == "" {
			return errors.Errorf("text content is required")
		}
		if tLen := utf8.RuneCountInString(r.Text); tLen > 50_000 {
			return errors.Errorf("text content is too long")
		}
	case sourcevo.SourceKindUrl:
		parsedUrl, err := url.ParseRequestURI(r.Url)
		if err != nil {
			return errors.Errorf("invalid url: %s", r.Url)
		}
		r.parsedUrl = parsedUrl
		if parsedUrl.Scheme != "http" && parsedUrl.Scheme != "https" {
			return errors.Errorf("invalid url scheme: %s", parsedUrl.Scheme)
		}
		// TODO safety issue, prevent url injection attacks
	}

	return nil
}

type CreateSourceResponse struct {
	Id string `json:"id"`
}

type UploadFileSourceRequest struct {
	Id uuid.UUID `path:"id,required"`

	MimeType string `json:"mime_type,required"`
	Filename string `json:"filename,required"  validate:"max=64"`
	Size     int64  `json:"size,required"      validate:"min=1"`
	Md5      string `json:"md5,required"       validate:"md5"`
}

const maxUploadFileSizeBytes int64 = 100 * 1024 * 1024 // 100MB

type UploadFileSourceResponse struct {
	Url     string            `json:"url"`
	Method  string            `json:"method"`
	Forms   map[string]string `json:"forms,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (r *UploadFileSourceRequest) Validate() error {
	if !sourceentity.SupportedFileMimeType(r.MimeType) {
		return errors.ErrParams.Msgf("unsupported mime_type: %s", r.MimeType)
	}
	if r.Size > maxUploadFileSizeBytes {
		return errors.ErrParams.Msg("file size exceeds")
	}

	return nil
}

type PollSourceStatusRequest struct {
	Id uuid.UUID `path:"id,required"`
}

type PollSourceStatusResponse struct {
	Status sourcevo.SourceStatus `json:"status"`
}

type RetrySourcePreparationRequest struct {
	Id uuid.UUID `path:"id,required"`
}

type DeleteSourceRequest struct {
	Id uuid.UUID `path:"id,required"`
}

type GetSourceDocRequest struct {
	Id    uuid.UUID `path:"id,required"` // source id
	DocId uuid.UUID `path:"doc_id,required"`
}

type SourceDocPosition struct {
	Start int `json:"start"`
	End   int `json:"end"`

	BytesStart int `json:"bytes_start"`
	BytesEnd   int `json:"bytes_end"`
}

type GetSourceDocResponse struct {
	SourceId    string `json:"source_id"`
	DocId       string `json:"doc_id"`
	SourceTitle string `json:"source_title"`
	Content     string `json:"content"`

	// 文档片段位置	rune offset 位置
	Position *SourceDocPosition `json:"position,omitempty"`
}

func ToGetSourceDocResponse(
	sourceId string,
	sourceTitle string,
	doc *sourceentity.SourceDoc,
) *GetSourceDocResponse {
	if doc == nil {
		return nil
	}

	resp := &GetSourceDocResponse{
		SourceId:    sourceId,
		DocId:       doc.Id.String(),
		SourceTitle: sourceTitle,
		Content:     doc.Content,
	}
	if doc.RunePos != nil {
		resp.Position = &SourceDocPosition{
			Start: doc.RunePos.GetStart(),
			End:   doc.RunePos.GetEnd(),
		}
	}
	if doc.BytePos != nil {
		resp.Position.BytesStart = doc.BytePos.GetStart()
		resp.Position.BytesEnd = doc.BytePos.GetEnd()
	}

	return resp
}

const maxBatchGetSourceDocsCount = 50

type BatchGetSourceDocsRequest struct {
	Id     uuid.UUID `path:"id,required"` // source id
	Ids    []string  `query:"ids,required"`
	docIds []valobj.Id
}

func (r *BatchGetSourceDocsRequest) DocIds() []valobj.Id {
	return r.docIds
}

func (r *BatchGetSourceDocsRequest) Validate() error {
	docIDs := make([]string, 0, len(r.Ids))
	for _, item := range r.Ids {
		for docID := range strings.SplitSeq(item, ",") {
			docID = strings.TrimSpace(docID)
			if docID == "" {
				continue
			}
			docIDs = append(docIDs, docID)
		}
	}

	if len(docIDs) == 0 {
		return errors.ErrParams.Msg("ids is required")
	}
	if len(docIDs) > maxBatchGetSourceDocsCount {
		return errors.ErrParams.Msgf("ids count exceeds limit: %d", maxBatchGetSourceDocsCount)
	}

	docIDs = slices.Unique(docIDs)
	docIds := make([]valobj.Id, 0, len(docIDs))
	for _, docID := range docIDs {
		id, err := valobj.NewIdFromString(docID)
		if err != nil {
			return errors.ErrParams.Msgf("invalid doc_id: %s", docID)
		}
		docIds = append(docIds, id)
	}

	r.Ids = docIDs
	r.docIds = docIds
	return nil
}

type BatchGetSourceDocsResponse struct {
	Docs []*GetSourceDocResponse `json:"docs"`
}

type GetSourceRequest struct {
	Id       uuid.UUID `path:"id,required"`
	Download bool      `query:"download,optional"`
}

type GetSourceParsedTreeRequest struct {
	Id uuid.UUID `path:"id,required"`
}

type UpdateSourceTitleRequest struct {
	Id    uuid.UUID `path:"id,required"`
	Title string    `json:"title" validate:"max=255"`
}

func (r *UpdateSourceTitleRequest) Validate() error {
	r.Title = strings.TrimSpace(r.Title)
	if r.Title == "" {
		return errors.ErrParams.Msg("source title is empty")
	}

	return nil
}
