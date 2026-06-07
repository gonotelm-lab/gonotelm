package schema

import "github.com/gonotelm-lab/gonotelm/internal/app/model"

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

func ToSources(sources []*model.FullSource) []*Source {
	resp := make([]*Source, 0, len(sources))
	for _, source := range sources {
		resp = append(resp, ToSource(source))
	}

	return resp
}
