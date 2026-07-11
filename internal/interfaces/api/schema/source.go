package schema

import (
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcevo "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
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
	Kind   sourcevo.SourceKind   `json:"kind"`
	Status sourcevo.SourceStatus `json:"status"`
	Title  string             `json:"title"`

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

