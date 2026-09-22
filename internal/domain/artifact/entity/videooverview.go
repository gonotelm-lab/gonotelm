package entity

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type VideoOverviewStyle string

const (
	VideoOverviewStyleDefault     VideoOverviewStyle = "default"
	VideoOverviewStyleEducational VideoOverviewStyle = "educational"
	VideoOverviewStyleCute        VideoOverviewStyle = "cute"
)

func (s VideoOverviewStyle) String() string { return string(s) }

func (s VideoOverviewStyle) Supported() bool {
	switch s {
	case VideoOverviewStyleDefault,
		VideoOverviewStyleEducational,
		VideoOverviewStyleCute:
		return true
	}
	return false
}

func VideoOverviewStyleDefaultValue() VideoOverviewStyle {
	return VideoOverviewStyleDefault
}

type VideoOverviewPayload struct {
	NotebookId  valobj.Id          `json:"notebook_id"`
	SourceIds   []valobj.Id        `json:"source_ids"`
	Tip         string             `json:"tip"`
	Language    Language           `json:"language"`
	VisualStyle VideoOverviewStyle `json:"visual_style"`
}

func (p *VideoOverviewPayload) Kind() Kind                { return KindVideoOverview }
func (p *VideoOverviewPayload) GetSourceIds() []valobj.Id { return p.SourceIds }

func (p *VideoOverviewPayload) GetTip() string {
	if p == nil {
		return ""
	}
	return p.Tip
}

func (p *VideoOverviewPayload) GetVisualStyle() VideoOverviewStyle {
	if p == nil || !p.VisualStyle.Supported() {
		return VideoOverviewStyleDefaultValue()
	}
	return p.VisualStyle
}

func (p *VideoOverviewPayload) GetLanguage() Language {
	if p == nil {
		return LanguageChinese
	}
	return p.Language
}
