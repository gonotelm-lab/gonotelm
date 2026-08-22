package schema

import (
	"unicode/utf8"

	audiooverview "github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/audiooverview"
	infographic "github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/infographic"
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

const maxUserTipLength = 300

type generatePayload interface {
	Validate() error
}

type GenerateArtifactRequest struct {
	NotebookId uuid.UUID   `path:"id,required"`
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
	Slides        *GenerateSlidesParameters        `json:"slides,omitempty"`

	// 保存为笔记
	Note *GenerateNoteParameters `json:"note,omitempty"`
}

func (r *GenerateArtifactRequest) Validate() error {
	if len(r.SourceIds) == 0 && r.Kind != artifactentity.KindNote {
		return errors.ErrParams.Msg("source_ids are required")
	}

	for _, item := range []struct {
		kind    Kind
		payload generatePayload
	}{
		{artifactentity.KindMindmap, asPayload(r.Mindmap)},
		{artifactentity.KindReport, asPayload(r.Report)},
		{artifactentity.KindInfoGraphic, asPayload(r.InfoGraphic)},
		{artifactentity.KindAudioOverview, asPayload(r.AudioOverview)},
		{artifactentity.KindFlashcard, asPayload(r.Flashcard)},
		{artifactentity.KindQuiz, asPayload(r.Quiz)},
		{artifactentity.KindDataTable, asPayload(r.DataTable)},
		{artifactentity.KindSlides, asPayload(r.Slides)},
		{artifactentity.KindNote, asPayload(r.Note)},
	} {
		if item.kind != r.Kind {
			continue
		}
		if item.payload == nil {
			return errors.ErrParams.Msgf("%s is required", r.Kind)
		}
		return item.payload.Validate()
	}

	return errors.ErrParams.Msgf("invalid kind: %s", r.Kind)
}

func asPayload[T any](p *T) generatePayload {
	if p == nil {
		return nil
	}
	return any(p).(generatePayload)
}

func validateUserTip(tip string) error {
	if utf8.RuneCountInString(tip) > maxUserTipLength {
		return errors.ErrParams.Msgf("tip exceeds %d characters", maxUserTipLength)
	}
	return nil
}

func validateLanguage(lang artifactentity.Language, allowEmpty bool) error {
	if allowEmpty && lang == "" {
		return nil
	}
	if !lang.IsValid() {
		return errors.ErrParams.Msgf("unsupported language: %s", lang)
	}
	return nil
}

type GenerateArtifactResponse struct {
	TaskId string `json:"task_id"`
}

type ArtifactTaskIdRequest struct {
	TaskId uuid.UUID `path:"id,required"`
}

type UpdateArtifactTarget string

const (
	UpdateArtifactTargetTitle UpdateArtifactTarget = "title"
)

type UpdateArtifactRequest struct {
	Id     uuid.UUID            `path:"id,required"`
	Target UpdateArtifactTarget `json:"target,required"`
	Title  string               `json:"title"`
}

func (r *UpdateArtifactRequest) Validate() error {
	switch r.Target {
	case UpdateArtifactTargetTitle:
		return nil
	default:
		return errors.ErrParams.Msgf("unsupported update target: %s", r.Target)
	}
}

type GetArtifactStatusResponse struct {
	TaskId string                `json:"task_id"`
	Status artifactentity.Status `json:"status"`
}

type ConvertNoteToSourceResponse struct {
	SourceId string `json:"source_id"`
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

func (p *GenerateMindmapParameters) Validate() error {
	return validateUserTip(p.Tip)
}

type GenerateReportParameters struct {
	Style    artifactentity.ReportStyle `json:"style,omitempty"`
	Language artifactentity.Language    `json:"language,omitempty"`
	Tip      string                     `json:"tip,omitempty"`
}

func (p *GenerateReportParameters) Validate() error {
	if err := validateLanguage(p.Language, false); err != nil {
		return err
	}
	return validateUserTip(p.Tip)
}

type GenerateInfoGraphicParameters struct {
	ExtraPrompt  string                                `json:"extra_prompt,omitempty"`
	TextLanguage string                                `json:"text_language,omitempty"`
	Orientation  artifactentity.InfoGraphicOrientation `json:"orientation,omitempty"`
	DetailLevel  artifactentity.InfoGraphicDetailLevel `json:"detail_level,omitempty"`
	VisualStyle  artifactentity.InfoGraphicVisualStyle `json:"visual_style,omitempty"`
}

func (p *GenerateInfoGraphicParameters) Validate() error {
	return validateUserTip(p.ExtraPrompt)
}

type GenerateAudioOverviewParameters struct {
	Tip      string                            `json:"tip,omitempty"`
	Language artifactentity.Language           `json:"language,omitempty"`
	Style    artifactentity.AudioOverviewStyle `json:"style,omitempty"`
}

func (p *GenerateAudioOverviewParameters) Validate() error {
	if err := validateLanguage(p.Language, false); err != nil {
		return err
	}
	return validateUserTip(p.Tip)
}

type GenerateFlashcardParameters struct {
	Count      artifactentity.FlashcardCount      `json:"count,omitempty"`
	Difficulty artifactentity.FlashcardDifficulty `json:"difficulty,omitempty"`
	Tip        string                             `json:"tip,omitempty"`
}

func (p *GenerateFlashcardParameters) Validate() error {
	if p.Count != "" && !p.Count.Supported() {
		return errors.ErrParams.Msgf("unsupported flashcard count: %s", p.Count)
	}
	if p.Difficulty != "" && !p.Difficulty.Supported() {
		return errors.ErrParams.Msgf("unsupported flashcard difficulty: %s", p.Difficulty)
	}
	return validateUserTip(p.Tip)
}

type GenerateQuizParameters struct {
	Count      artifactentity.QuizCount      `json:"count,omitempty"`
	Difficulty artifactentity.QuizDifficulty `json:"difficulty,omitempty"`
	Tip        string                        `json:"tip,omitempty"`
}

func (p *GenerateQuizParameters) Validate() error {
	if p.Count != "" && !p.Count.Supported() {
		return errors.ErrParams.Msgf("unsupported quiz count: %s", p.Count)
	}
	if p.Difficulty != "" && !p.Difficulty.Supported() {
		return errors.ErrParams.Msgf("unsupported quiz difficulty: %s", p.Difficulty)
	}
	return validateUserTip(p.Tip)
}

type GenerateDataTableParameters struct {
	Tip string `json:"tip,omitempty"`
}

func (p *GenerateDataTableParameters) Validate() error {
	return validateUserTip(p.Tip)
}

type GenerateSlidesParameters struct {
	Tip         string                           `json:"tip,omitempty"`
	VisualStyle artifactentity.SlidesVisualStyle `json:"visual_style,omitempty"`
	Language    artifactentity.Language          `json:"language,omitempty"`
}

func (p *GenerateSlidesParameters) Validate() error {
	if err := validateLanguage(p.Language, true); err != nil {
		return err
	}
	if p.VisualStyle != "" && !p.VisualStyle.Supported() {
		return errors.ErrParams.Msgf("unsupported slides visual_style: %s", p.VisualStyle)
	}
	return validateUserTip(p.Tip)
}

// 将对话内容保存为笔记
type GenerateNoteParameters struct {
	ChatId uuid.UUID `json:"chat_id,required"`
	MsgId  uuid.UUID `json:"msg_id,required"`
}

func (p *GenerateNoteParameters) Validate() error {
	return nil
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

type SlidesExtras struct {
	Tip         string `json:"tip"`
	VisualStyle string `json:"visual_style"`
	Language    string `json:"language"`
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
	case *artifactentity.SlidesPayload:
		r.SourceIds = p.SourceIds
		r.Extras = &SlidesExtras{
			Tip:         p.Tip,
			VisualStyle: p.GetVisualStyle().String(),
			Language:    string(p.GetLanguage()),
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

func (r *GenerateSlidesParameters) ToPayload() *artifactentity.SlidesPayload {
	if r == nil {
		return nil
	}

	return &artifactentity.SlidesPayload{
		Tip:         r.Tip,
		VisualStyle: r.VisualStyle,
		Language:    r.Language,
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
