package prompt

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
)

type Prompt struct {
	defaultLang string
	systemMsg   *schema.Message
}

func New(defaultLang string) *Prompt {
	normalizedLang := normalizeTemplateLang(defaultLang)
	return &Prompt{
		defaultLang: normalizedLang,
		systemMsg:   schema.SystemMessage(systemPrompt),
	}
}

func (p *Prompt) RenderStudioMindmapV2Message(ctx context.Context, sourceIds []string, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[StudioMindmapV2TemplateVars](templateNameStudioMindmapV2, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, StudioMindmapV2TemplateVars{SourceIds: sourceIds})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

func (p *Prompt) RenderStudioReportMessage(ctx context.Context, sourceIds []string, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[StudioReportTemplateVars](templateNameStudioReport, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, StudioReportTemplateVars{SourceIds: sourceIds})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

func (p *Prompt) RenderStudioInfoGraphicMessage(ctx context.Context, vars StudioInfoGraphicTemplateVars, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[StudioInfoGraphicTemplateVars](templateNameStudioInfographic, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, vars)
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

func (p *Prompt) RenderStudioPodcastOutlineMessage(ctx context.Context, sourceIds []string, lang string, tips string, style PodcastStyle) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[StudioPodcastOutlineTemplateVars](templateNameStudioPodcastOutline, lang, p.defaultLang)
	vars := StudioPodcastOutlineTemplateVars{
		SourceIds: sourceIds,
		Language:  lang,
		Tips:      tips,
	}
	info, ok := builtinPodcastInfos[style]
	if !ok {
		info = builtinPodcastInfos[PodcastStyleAbstract]
	}
	vars.Style = info.Style
	vars.StyleDesc = info.Description
	vars.Speakers = info.Speakers
	vars.NumOfSegments = info.NumOfSegments

	msg, err := tmpl.Message(ctx, vars)
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

func (p *Prompt) RenderTitleMakerMessage(ctx context.Context, text, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[TitleMakerTemplateVars](templateNameTitleMaker, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, TitleMakerTemplateVars{Text: text})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

func (p *Prompt) prependSystemMessage(msgs []*schema.Message) []*schema.Message {
	return append([]*schema.Message{p.systemMsg}, msgs...)
}

func newPromptTemplate[T templateVars](tmplName templateName, lang, defaultLang string) *template[T] {
	normalizedName := normalizeTemplateName(tmplName)
	normalizedLang := strings.TrimSpace(lang)
	if normalizedLang == "" {
		normalizedLang = defaultLang
	}
	content := readTemplate(normalizedName, normalizedLang)
	return &template[T]{
		name: normalizedName,
		lang: normalizedLang,
		tmpl: fromMessages(schema.Jinja2, schema.UserMessage(content)),
	}
}
