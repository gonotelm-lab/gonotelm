package studio

import (
	audiooverview "github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/audiooverview"
	infographic "github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/infographic"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
)

type (
	Kind       = artifactentity.Kind
	Status     = artifactentity.Status
	ResultKind = artifactentity.ResultKind
)

type GenerateArtifactRequest struct {
	NotebookId uuid.UUID   `json:"notebook_id,required"`
	Kind       Kind        `json:"kind,required"`
	SourceIds  []uuid.UUID `json:"source_ids"`

	// 工作区生成产物
	Mindmap       *GenerateMindmapParameters       `json:"mindmap,omitempty"`
	Report        *GenerateReportParameters        `json:"report,omitempty"`
	InfoGraphic   *GenerateInfoGraphicParameters   `json:"info_graphic,omitempty"`
	AudioOverview *GenerateAudioOverviewParameters `json:"audio_overview,omitempty"`
	Flashcard     *GenerateFlashcardParameters     `json:"flashcard,omitempty"`
	Quiz          *GenerateQuizParameters          `json:"quiz,omitempty"`
	DataTable     *GenerateDataTableParameters     `json:"data_table,omitempty"`

	// 保存为笔记
	Note *GenerateNoteParameters `json:"note,omitempty"`
}

func (r *GenerateArtifactRequest) Validate() error {
	// Kind → 对应 payload 指针；校验「kind 合法」且「匹配字段非 nil」
	required := map[Kind]func() any{
		artifactentity.KindMindmap:       func() any { return r.Mindmap },
		artifactentity.KindReport:        func() any { return r.Report },
		artifactentity.KindInfoGraphic:   func() any { return r.InfoGraphic },
		artifactentity.KindAudioOverview: func() any { return r.AudioOverview },
		artifactentity.KindFlashcard:     func() any { return r.Flashcard },
		artifactentity.KindQuiz:          func() any { return r.Quiz },
		artifactentity.KindDataTable:     func() any { return r.DataTable },
		artifactentity.KindNote:          func() any { return r.Note },
	}

	get, ok := required[r.Kind]
	if !ok {
		return errors.ErrParams.Msgf("invalid kind: %s", r.Kind)
	}

	if get() == nil {
		return errors.ErrParams.Msgf("%s is required", r.Kind)
	}

	if len(r.SourceIds) == 0 && r.Kind != artifactentity.KindNote {
		return errors.ErrParams.Msg("source_ids are required")
	}

	return nil
}

type GenerateArtifactResponse struct {
	TaskId string `json:"task_id"`
}

type ArtifactTaskIdRequest struct {
	TaskId uuid.UUID `path:"task_id,required"`
}

type GetArtifactStatusResponse struct {
	TaskId string                `json:"task_id"`
	Status artifactentity.Status `json:"status"`
}

type ListNotebookArtifactsRequest struct {
	Id     uuid.UUID `path:"id,required"`
	Limit  int       `query:"limit"`
	Offset int       `query:"offset"`
}

type ListNotebookArtifactsResponse struct {
	Artifacts []*ArtifactItem `json:"artifacts"`
	Limit     int             `json:"limit"`
	Offset    int             `json:"offset"`
	HasMore   bool            `json:"has_more"`
}

type GenerateMindmapParameters struct {
	Tip string `json:"tip,omitempty"`
}

type GenerateReportParameters struct {
	Style    artifactentity.ReportStyle `json:"style,omitempty"`
	Language artifactentity.Language    `json:"language,omitempty"`
	Tip      string                     `json:"tip,omitempty"`
}

type GenerateInfoGraphicParameters struct {
	ExtraPrompt  string                                `json:"extra_prompt,omitempty"`
	TextLanguage string                                `json:"text_language,omitempty"`
	Orientation  artifactentity.InfoGraphicOrientation `json:"orientation,omitempty"`
	DetailLevel  artifactentity.InfoGraphicDetailLevel `json:"detail_level,omitempty"`
	VisualStyle  artifactentity.InfoGraphicVisualStyle `json:"visual_style,omitempty"`
}

type GenerateAudioOverviewParameters struct {
	Tip      string                            `json:"tip,omitempty"`
	Language artifactentity.Language           `json:"language,omitempty"`
	Style    artifactentity.AudioOverviewStyle `json:"style,omitempty"`
}

type GenerateFlashcardParameters struct {
	Count      artifactentity.FlashcardCount      `json:"count,omitempty"`
	Difficulty artifactentity.FlashcardDifficulty `json:"difficulty,omitempty"`
	Tip        string                             `json:"tip,omitempty"`
}

type GenerateQuizParameters struct {
	Count      artifactentity.QuizCount      `json:"count,omitempty"`
	Difficulty artifactentity.QuizDifficulty `json:"difficulty,omitempty"`
	Tip        string                        `json:"tip,omitempty"`
}

type GenerateDataTableParameters struct {
	Tip string `json:"tip,omitempty"`
}

// 将对话内容保存为笔记
type GenerateNoteParameters struct {
	ChatId uuid.UUID `json:"chat_id,required"`
	MsgId  uuid.UUID `json:"msg_id,required"`
}

type ArtifactItem struct {
	NotebookId  string                   `json:"notebook_id"`
	TaskId      string                   `json:"task_id"`
	Kind        string                   `json:"kind"`
	Status      string                   `json:"status"`
	Title       string                   `json:"title"`
	SourceIds   []uuid.UUID              `json:"source_ids,omitempty"`
	Timestamp   int64                    `json:"timestamp"`
	Content     string                   `json:"content,omitempty"`
	ContentUrl  string                   `json:"content_url,omitempty"`
	ContentKind string                   `json:"content_kind"`
	MimeType    string                   `json:"mime_type"`
	ImageInfo   *ArtifactResultImageInfo `json:"image_info,omitempty"`
	Extras      any                      `json:"extras,omitempty"`
}

type ArtifactResultImageInfo struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type MindmapExtras struct {
	Tip string `json:"tip"`
}

type ReportExtras struct {
	Style    string `json:"style"`
	Language string `json:"language"`
	Tip      string `json:"tip"`
}

type InfoGraphicExtras struct {
	Prompt       string `json:"prompt"`
	TextLanguage string `json:"text_language"`
	Orientation  string `json:"orientation"`
	DetailLevel  string `json:"detail_level"`
	VisualStyle  string `json:"visual_style"`
}

type AudioOverviewExtras struct {
	Tip        string `json:"tip"`
	Language   string `json:"language"`
	Style      string `json:"style"`
	Format     string `json:"format,omitempty"`
	Channels   int    `json:"channels,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

type FlashcardExtras struct {
	Count      string `json:"count"`
	Difficulty string `json:"difficulty"`
	Tip        string `json:"tip"`
}

type QuizExtras struct {
	Count      string `json:"count"`
	Difficulty string `json:"difficulty"`
	Tip        string `json:"tip"`
}

type DataTableExtras struct {
	Tip string `json:"tip"`
}

type NoteExtras struct {
	ChatId uuid.UUID `json:"chat_id"`
	MsgId  uuid.UUID `json:"msg_id"`
}

func ToArtifactItem(a *artifactentity.Artifact) *ArtifactItem {
	r := &ArtifactItem{
		NotebookId:  a.NotebookId.String(),
		TaskId:      a.Id.String(),
		Kind:        a.Kind.String(),
		Status:      a.Status.String(),
		Title:       a.Title,
		Timestamp:   a.CreateTime.Value(),
		ContentKind: a.ResultKind.String(),
	}

	switch p := a.Payload.(type) {
	case *artifactentity.MindmapPayload:
		r.SourceIds = p.SourceIds
		r.Extras = &MindmapExtras{
			Tip: p.Tip,
		}
	case *artifactentity.ReportPayload:
		r.SourceIds = p.SourceIds
		r.Extras = &ReportExtras{
			Style:    string(p.Style),
			Language: string(p.Language),
			Tip:      p.Tip,
		}
	case *artifactentity.InfoGraphicPayload:
		r.SourceIds = p.SourceIds
		r.Extras = &InfoGraphicExtras{
			Prompt:       p.ExtraPrompt,
			TextLanguage: p.TextLanguage,
			Orientation:  p.Orientation.String(),
			DetailLevel:  p.DetailLevel.String(),
			VisualStyle:  p.VisualStyle.String(),
		}
	case *artifactentity.AudioOverviewPayload:
		r.SourceIds = p.SourceIds
		r.Extras = &AudioOverviewExtras{
			Tip:      p.Tip,
			Language: string(p.Language),
			Style:    string(p.Style),
		}
	case *artifactentity.FlashcardPayload:
		r.SourceIds = p.SourceIds
		r.Extras = &FlashcardExtras{
			Count:      string(p.Count),
			Difficulty: string(p.Difficulty),
			Tip:        p.Tip,
		}
	case *artifactentity.QuizPayload:
		r.SourceIds = p.SourceIds
		r.Extras = &QuizExtras{
			Count:      string(p.Count),
			Difficulty: string(p.Difficulty),
			Tip:        p.Tip,
		}
	case *artifactentity.DataTablePayload:
		r.SourceIds = p.SourceIds
		r.Extras = &DataTableExtras{
			Tip: p.Tip,
		}
	case *artifactentity.NotePayload:
		r.Extras = &NoteExtras{
			ChatId: p.ChatId,
			MsgId:  p.MsgId,
		}
	}

	if a.ResultKind.Inline() && a.Result != nil {
		r.Content = string(a.Result)
	}
	if a.ResultKind.Storage() && a.Result != nil {
		switch a.Kind {
		case artifactentity.KindInfoGraphic:
			var sr infographic.StorageResult
			if sonic.Unmarshal(a.Result, &sr) == nil {
				r.MimeType = sr.ContentType
				if sr.Image != nil {
					r.ImageInfo = &ArtifactResultImageInfo{
						Width:  sr.Image.Width,
						Height: sr.Image.Height,
					}
				}
			}
		case artifactentity.KindAudioOverview:
			var sr audiooverview.AudioStorageResult
			if sonic.Unmarshal(a.Result, &sr) == nil {
				r.MimeType = sr.ContentType
				if sr.Audio != nil {
					if r.Extras == nil {
						r.Extras = &AudioOverviewExtras{}
					}
					extra := r.Extras.(*AudioOverviewExtras)
					extra.DurationMs = sr.Audio.DurationMs
					extra.Format = sr.Audio.Format
					extra.Channels = sr.Audio.NumChannels
					extra.SampleRate = sr.Audio.SampleRate
				}
			}
		}
	}

	return r
}

func ToArtifactItems(artifacts []*artifactentity.Artifact) []*ArtifactItem {
	results := make([]*ArtifactItem, 0, len(artifacts))
	for _, a := range artifacts {
		results = append(results, ToArtifactItem(a))
	}
	return results
}

func (r *GenerateMindmapParameters) ToPayload() *artifactentity.MindmapPayload {
	if r == nil {
		return nil
	}

	return &artifactentity.MindmapPayload{
		Tip: r.Tip,
	}
}

func (r *GenerateReportParameters) ToPayload() *artifactentity.ReportPayload {
	if r == nil {
		return nil
	}

	return &artifactentity.ReportPayload{
		Style:    r.Style,
		Language: r.Language,
		Tip:      r.Tip,
	}
}

func (r *GenerateInfoGraphicParameters) ToPayload() *artifactentity.InfoGraphicPayload {
	if r == nil {
		return nil
	}

	return &artifactentity.InfoGraphicPayload{
		ExtraPrompt:  r.ExtraPrompt,
		TextLanguage: r.TextLanguage,
		Orientation:  r.Orientation,
		DetailLevel:  r.DetailLevel,
		VisualStyle:  r.VisualStyle,
	}
}

func (r *GenerateAudioOverviewParameters) ToPayload() *artifactentity.AudioOverviewPayload {
	if r == nil {
		return nil
	}

	return &artifactentity.AudioOverviewPayload{
		Tip:      r.Tip,
		Language: r.Language,
		Style:    r.Style,
	}
}

func (r *GenerateFlashcardParameters) ToPayload() *artifactentity.FlashcardPayload {
	if r == nil {
		return nil
	}

	return &artifactentity.FlashcardPayload{
		Count:      r.Count,
		Difficulty: r.Difficulty,
		Tip:        r.Tip,
	}
}

func (r *GenerateQuizParameters) ToPayload() *artifactentity.QuizPayload {
	if r == nil {
		return nil
	}

	return &artifactentity.QuizPayload{
		Count:      r.Count,
		Difficulty: r.Difficulty,
		Tip:        r.Tip,
	}
}

func (r *GenerateDataTableParameters) ToPayload() *artifactentity.DataTablePayload {
	if r == nil {
		return nil
	}

	return &artifactentity.DataTablePayload{
		Tip: r.Tip,
	}
}

func (r *GenerateNoteParameters) ToPayload() *artifactentity.NotePayload {
	if r == nil {
		return nil
	}

	return &artifactentity.NotePayload{
		ChatId: r.ChatId,
		MsgId:  r.MsgId,
	}
}
