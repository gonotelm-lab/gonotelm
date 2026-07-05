package schema

import (
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
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
	Id     string             `json:"id"`
	Kind   model.SourceKind   `json:"kind"`
	Status model.SourceStatus `json:"status"`
	Title  string             `json:"title"`

	Text *TextSourceContent `json:"text,omitempty"`
	Url  *UrlSourceContent  `json:"url,omitempty"`
	File *FileSourceContent `json:"file,omitempty"`

	ParsedContent *SourceParsedContent `json:"parsed_content,omitempty"`
}

type SourceParsedContent struct {
	Url string `json:"url,omitempty"`
}

func ToSource(source *model.FullSource) *Source {
	s := &Source{
		Id:     source.Source.Id.String(),
		Kind:   source.Source.Kind,
		Status: source.Source.Status,
		Title:  source.Source.Title,
		ParsedContent: &SourceParsedContent{
			Url: source.ParsedContentUrl,
		},
	}

	if source.Source.Kind.IsText() {
		s.Text = &TextSourceContent{
			Text: source.DecodedSource.ContentText.Text,
		}
	}
	if source.Source.Kind.IsUrl() {
		s.Url = &UrlSourceContent{
			Url: source.DecodedSource.ContentUrl.Url,
		}
	}
	if source.Source.Kind.IsFile() {
		s.File = &FileSourceContent{
			Filename: source.DecodedSource.ContentFile.Filename,
			Format:   source.DecodedSource.ContentFile.Format,
			Url:      source.DecodedSource.ContentFile.Url,
		}
	}

	return s
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
		Kind:   model.SourceKind(source.Kind),
		Status: model.SourceStatus(source.Status),
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

func ToSources(sources []*model.FullSource) []*Source {
	resp := make([]*Source, 0, len(sources))
	for _, source := range sources {
		resp = append(resp, ToSource(source))
	}

	return resp
}
