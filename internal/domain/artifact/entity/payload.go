package entity

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type Payload interface {
	Kind() Kind
}

type MindmapPayload struct {
	NotebookId valobj.Id   `json:"notebook_id"`
	SourceIds  []valobj.Id `json:"source_ids"`
}

func (p *MindmapPayload) Kind() Kind { return KindMindmap }

type ReportPayload struct {
	NotebookId valobj.Id   `json:"notebook_id"`
	SourceIds  []valobj.Id `json:"source_ids"`
}

func (p *ReportPayload) Kind() Kind { return KindReport }

type InfoGraphicPayload struct {
	NotebookId   valobj.Id                      `json:"notebook_id"`
	SourceIds    []valobj.Id                    `json:"source_ids"`
	ExtraPrompt  string                         `json:"extra_prompt"`
	TextLanguage string                         `json:"text_language"`
	Orientation  ArtifactInfoGraphicOrientation `json:"orientation"`
	DetailLevel  ArtifactInfoGraphicDetailLevel `json:"detail_level"`
}

func (p *InfoGraphicPayload) Kind() Kind { return KindInfoGraphic }

type AudioOverviewPayload struct {
	NotebookId valobj.Id                `json:"notebook_id"`
	SourceIds  []valobj.Id              `json:"source_ids"`
	Tip        string                   `json:"tip"`
	Language   string                   `json:"language"`
	Style      ArtifactAudioOverviewStyle `json:"style"`
}

func (p *AudioOverviewPayload) Kind() Kind { return KindAudioOverview }