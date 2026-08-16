package entity

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type SlidesPayload struct {
	NotebookId valobj.Id   `json:"notebook_id"`
	SourceIds  []valobj.Id `json:"source_ids"`
	Tip        string      `json:"tip"`
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
