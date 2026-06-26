package prompt

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// Prompt manages prompt template rendering. Template files are loaded
// from embedded FS at construction via New().
type Prompt struct {
	store       preloadedTemplates
	defaultLang string
	systemMsg   *schema.Message
	chatManager *ChatTemplateManager
}

// New creates a Prompt with the given default language.
// Template files are embedded and loaded at construction time.
func New(defaultLang string) *Prompt {
	normalizedLang := normalizeTemplateLang(defaultLang)
	chatManager, err := NewChatTemplateManager(normalizedLang)
	if err != nil {
		panic("initialize chat template manager failed: " + err.Error())
	}

	return &Prompt{
		store:       templateStore,
		defaultLang: normalizedLang,
		systemMsg:   schema.SystemMessage(systemPrompt),
		chatManager: chatManager,
	}
}

// ChatTemplate returns the chat template for the given language.
func (p *Prompt) ChatTemplate(lang string) *ChatTemplate {
	return p.chatManager.Get(lang)
}

// RenderSummarizeMessage renders a summarization prompt.
func (p *Prompt) RenderSummarizeMessage(ctx context.Context, text, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[SummarizeTemplateVars](templateNameSummarize, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, SummarizeTemplateVars{Text: text})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

// RenderTitleMakerMessage renders a title generation prompt.
func (p *Prompt) RenderTitleMakerMessage(ctx context.Context, text, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[TitleMakerTemplateVars](templateNameTitleMaker, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, TitleMakerTemplateVars{Text: text})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

// RenderRetrieveSourceDocMessage renders a source document retrieval prompt.
func (p *Prompt) RenderRetrieveSourceDocMessage(
	ctx context.Context,
	question string,
	notebookId string,
	sources []*RetrieveSource,
	lang string,
) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[RetrieveSourceDocTemplateVars](templateNameRetrieveSourceDoc, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, RetrieveSourceDocTemplateVars{
		Question:   question,
		NotebookId: notebookId,
		Sources:    sources,
	})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

// RenderNotebookSummaryMessage renders a notebook summary prompt.
func (p *Prompt) RenderNotebookSummaryMessage(ctx context.Context, sources []string, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[NotebookSummaryTemplateVars](templateNameNotebookSummary, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, NotebookSummaryTemplateVars{Sources: sources})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

// RenderStudioMindmapContentMessage renders a studio mindmap prompt in content mode.
func (p *Prompt) RenderStudioMindmapContentMessage(ctx context.Context, contents []string, lang string) ([]*schema.Message, error) {
	return p.renderStudioMindmapMessageWithMode(ctx, StudioMindmapModeContent, contents, nil, lang)
}

// RenderStudioMindmapAbstractMessage renders a studio mindmap prompt in abstract mode.
func (p *Prompt) RenderStudioMindmapAbstractMessage(ctx context.Context, abstracts []string, lang string) ([]*schema.Message, error) {
	return p.renderStudioMindmapMessageWithMode(ctx, StudioMindmapModeAbstract, nil, abstracts, lang)
}

// RenderStudioMindmapMessageWithMode renders a studio mindmap prompt with the given mode.
func (p *Prompt) RenderStudioMindmapMessageWithMode(
	ctx context.Context,
	mode string,
	contents []string,
	abstracts []string,
	lang string,
) ([]*schema.Message, error) {
	return p.renderStudioMindmapMessageWithMode(ctx, mode, contents, abstracts, lang)
}

func (p *Prompt) renderStudioMindmapMessageWithMode(
	ctx context.Context,
	mode string,
	contents []string,
	abstracts []string,
	lang string,
) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[StudioMindmapTemplateVars](templateNameStudioMindmap, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, StudioMindmapTemplateVars{
		Mode:      mode,
		Contents:  contents,
		Abstracts: abstracts,
	})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

// RenderStudioMindmapV2Message renders a studio mindmap v2 prompt.
func (p *Prompt) RenderStudioMindmapV2Message(ctx context.Context, sourceIds []string, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[StudioMindmapV2TemplateVars](templateNameStudioMindmapV2, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, StudioMindmapV2TemplateVars{SourceIds: sourceIds})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

// RenderStudioReportMessage renders a studio report prompt.
func (p *Prompt) RenderStudioReportMessage(ctx context.Context, sourceIds []string, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[StudioReportTemplateVars](templateNameStudioReport, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, StudioReportTemplateVars{SourceIds: sourceIds})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

// RenderStudioInfoGraphicMessage renders a studio infographic prompt.
func (p *Prompt) RenderStudioInfoGraphicMessage(ctx context.Context, vars StudioInfoGraphicTemplateVars, lang string) ([]*schema.Message, error) {
	tmpl := newPromptTemplate[StudioInfoGraphicTemplateVars](templateNameStudioInfographic, lang, p.defaultLang)
	msg, err := tmpl.Message(ctx, vars)
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

// RenderStudioPodcastOutlineMessage renders a studio podcast outline prompt.
func (p *Prompt) RenderStudioPodcastOutlineMessage(
	ctx context.Context,
	sourceIds []string,
	lang string,
	tips string,
	style PodcastStyle,
) ([]*schema.Message, error) {
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

func (p *Prompt) prependSystemMessage(msgs []*schema.Message) []*schema.Message {
	return append([]*schema.Message{p.systemMsg}, msgs...)
}

// newPromptTemplate creates a template from the pre-loaded store, falling
// back to defaultLang when the requested lang is empty.
func newPromptTemplate[T templateVars](tmplName templateName, lang, defaultLang string) *template[T] {
	normalizedName := normalizeTemplateName(tmplName)
	normalizedLang := normalizeTemplateLang(lang)
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
