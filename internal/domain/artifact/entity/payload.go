package entity

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type Payload interface {
	Kind() Kind
	GetSourceIds() []valobj.Id
}

func PayloadAs[T any](p Payload) T {
	return p.(T)
}

type MindmapPayload struct {
	NotebookId valobj.Id   `json:"notebook_id"`
	SourceIds  []valobj.Id `json:"source_ids"`
	Tip        string      `json:"tip"`
}

func (p *MindmapPayload) Kind() Kind                { return KindMindmap }
func (p *MindmapPayload) GetSourceIds() []valobj.Id { return p.SourceIds }

func (p *MindmapPayload) GetTip() string {
	if p == nil {
		return ""
	}
	return p.Tip
}

type DataTablePayload struct {
	NotebookId valobj.Id   `json:"notebook_id"`
	SourceIds  []valobj.Id `json:"source_ids"`
	Tip        string      `json:"tip"`
}

func (p *DataTablePayload) Kind() Kind                { return KindDataTable }
func (p *DataTablePayload) GetSourceIds() []valobj.Id { return p.SourceIds }

func (p *DataTablePayload) GetTip() string {
	if p == nil {
		return ""
	}
	return p.Tip
}

type ReportStyle string

const (
	ReportStyleDefault    ReportStyle = "default"
	ReportStyleBrief      ReportStyle = "brief"
	ReportStyleStudyGuide ReportStyle = "study-guide"
	ReportStyleDetailed   ReportStyle = "detailed"
)

func (s ReportStyle) String() string { return string(s) }
func (s ReportStyle) Supported() bool {
	switch s {
	case ReportStyleDefault, ReportStyleBrief, ReportStyleStudyGuide, ReportStyleDetailed:
		return true
	}
	return false
}

func ReportStyleDefaultStyle() ReportStyle {
	return ReportStyleDefault
}

type ReportPayload struct {
	NotebookId valobj.Id   `json:"notebook_id"`
	SourceIds  []valobj.Id `json:"source_ids"`
	Style      ReportStyle `json:"style"`
	Language   Language    `json:"language"`
	Tip        string      `json:"tip"`
}

func (p *ReportPayload) Kind() Kind                { return KindReport }
func (p *ReportPayload) GetSourceIds() []valobj.Id { return p.SourceIds }

func (p *ReportPayload) GetTip() string {
	if p == nil {
		return ""
	}
	return p.Tip
}

type InfoGraphicPayload struct {
	NotebookId   valobj.Id              `json:"notebook_id"`
	SourceIds    []valobj.Id            `json:"source_ids"`
	ExtraPrompt  string                 `json:"extra_prompt"`
	TextLanguage string                 `json:"text_language"`
	Orientation  InfoGraphicOrientation `json:"orientation"`
	DetailLevel  InfoGraphicDetailLevel `json:"detail_level"`
	VisualStyle  InfoGraphicVisualStyle `json:"visual_style"`
}

func (p *InfoGraphicPayload) Kind() Kind                { return KindInfoGraphic }
func (p *InfoGraphicPayload) GetSourceIds() []valobj.Id { return p.SourceIds }

type AudioOverviewPayload struct {
	NotebookId valobj.Id          `json:"notebook_id"`
	SourceIds  []valobj.Id        `json:"source_ids"`
	Tip        string             `json:"tip"`
	Language   Language           `json:"language"`
	Style      AudioOverviewStyle `json:"style"`
}

func (p *AudioOverviewPayload) Kind() Kind                { return KindAudioOverview }
func (p *AudioOverviewPayload) GetSourceIds() []valobj.Id { return p.SourceIds }

func (p *AudioOverviewPayload) GetTip() string {
	if p == nil {
		return ""
	}
	return p.Tip
}

type NotePayload struct {
	ChatId valobj.Id `json:"chat_id"`
	MsgId  valobj.Id `json:"msg_id"`
}

func (p *NotePayload) Kind() Kind                { return KindNote }
func (p *NotePayload) GetSourceIds() []valobj.Id { return nil }
