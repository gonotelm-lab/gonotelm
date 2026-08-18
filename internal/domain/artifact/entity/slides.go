package entity

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type SlidesVisualStyle string

const (
	SlidesVisualStyleDefault     SlidesVisualStyle = "default"
	SlidesVisualStyleEducational SlidesVisualStyle = "educational"
	SlidesVisualStyleCute        SlidesVisualStyle = "cute"
)

func (s SlidesVisualStyle) String() string { return string(s) }

func (s SlidesVisualStyle) Supported() bool {
	switch s {
	case SlidesVisualStyleDefault,
		SlidesVisualStyleEducational,
		SlidesVisualStyleCute:
		return true
	}
	return false
}

func SlidesVisualStyleDefaultValue() SlidesVisualStyle {
	return SlidesVisualStyleDefault
}

type SlidesPayload struct {
	NotebookId  valobj.Id         `json:"notebook_id"`
	SourceIds   []valobj.Id       `json:"source_ids"`
	Tip         string            `json:"tip"`
	VisualStyle SlidesVisualStyle `json:"visual_style"`
	Language    Language          `json:"language"`
}

func (p *SlidesPayload) Kind() Kind                { return KindSlides }
func (p *SlidesPayload) GetSourceIds() []valobj.Id { return p.SourceIds }

func (s *SlidesPayload) GetNotebookId() valobj.Id {
	if s == nil {
		return valobj.Id{}
	}

	return s.NotebookId
}

func (s *SlidesPayload) GetTip() string {
	if s == nil {
		return ""
	}

	return s.Tip
}

func (s *SlidesPayload) GetVisualStyle() SlidesVisualStyle {
	if s == nil || !s.VisualStyle.Supported() {
		return SlidesVisualStyleDefaultValue()
	}
	return s.VisualStyle
}

func (s *SlidesPayload) GetLanguage() Language {
	if s == nil {
		return LanguageAuto
	}
	return s.Language
}
