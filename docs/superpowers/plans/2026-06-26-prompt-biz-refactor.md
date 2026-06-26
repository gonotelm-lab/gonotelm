# Prompt Biz Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move `internal/app/prompt/` into `internal/app/biz/prompt/`, wrapping package-level state/functions into a `Prompt` struct.

**Architecture:** Create `biz/prompt/prompt.go` with a `Prompt` struct holding the pre-loaded template store, system message, and chat template manager. All `Render*` functions and `ChatTemplateManager` become methods. Update 10 consumer files.

**Tech Stack:** Go 1.25, embed FS for `.jinja` templates, `github.com/cloudwego/eino`

## Global Constraints

- No changes to `.jinja` template file content
- No changes to the generic `template[T]` rendering engine logic
- All existing type names and method signatures preserved (only receiver changes)
- `CheckStudioMindmapResult` stays as a standalone function (no state needed)
- Compilation must pass after each task

---

### Task 1: Create biz/prompt directory and move source files

**Files:**
- Create: `internal/app/biz/prompt/` (directory)
- Move: `internal/app/prompt/*.go` → `internal/app/biz/prompt/*.go` (all 13 .go files)
- Move: `internal/app/prompt/zh/*.jinja` → `internal/app/biz/prompt/zh/*.jinja` (all 11 .jinja files)
- Delete: `internal/app/prompt/` (old directory, after move)

- [ ] **Step 1: Create target directory and move files**

```bash
mkdir -p internal/app/biz/prompt/zh
```

- [ ] **Step 2: Move all .go source files**

```bash
mv internal/app/prompt/*.go internal/app/biz/prompt/
```

- [ ] **Step 3: Move all .jinja template files**

```bash
mv internal/app/prompt/zh/*.jinja internal/app/biz/prompt/zh/
```

- [ ] **Step 4: Remove old empty directory**

```bash
rmdir internal/app/prompt/zh && rmdir internal/app/prompt
```

- [ ] **Step 5: Verify files are in place**

```bash
ls internal/app/biz/prompt/*.go internal/app/biz/prompt/zh/*.jinja | wc -l
```
Expected: 24 (13 .go files + 11 .jinja files)

- [ ] **Step 6: Commit**

```bash
git add internal/app/biz/prompt/ internal/app/prompt/
git commit -m "refactor: move prompt package from internal/app/prompt to internal/app/biz/prompt"
```

---

### Task 2: Create prompt.go with Prompt struct and New() constructor

**Files:**
- Create: `internal/app/biz/prompt/prompt.go`

**Interfaces:**
- Produces: `type Prompt struct`, `func New(defaultLang string) *Prompt`, all `Render*` methods on `*Prompt`

- [ ] **Step 1: Write prompt.go**

```go
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
	tmpl := p.newTemplate[SummarizeTemplateVars](templateNameSummarize, lang)
	msg, err := tmpl.Message(ctx, SummarizeTemplateVars{Text: text})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

// RenderTitleMakerMessage renders a title generation prompt.
func (p *Prompt) RenderTitleMakerMessage(ctx context.Context, text, lang string) ([]*schema.Message, error) {
	tmpl := p.newTemplate[TitleMakerTemplateVars](templateNameTitleMaker, lang)
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
	tmpl := p.newTemplate[RetrieveSourceDocTemplateVars](templateNameRetrieveSourceDoc, lang)
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
	tmpl := p.newTemplate[NotebookSummaryTemplateVars](templateNameNotebookSummary, lang)
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
	tmpl := p.newTemplate[StudioMindmapTemplateVars](templateNameStudioMindmap, lang)
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
	tmpl := p.newTemplate[StudioMindmapV2TemplateVars](templateNameStudioMindmapV2, lang)
	msg, err := tmpl.Message(ctx, StudioMindmapV2TemplateVars{SourceIds: sourceIds})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

// RenderStudioReportMessage renders a studio report prompt.
func (p *Prompt) RenderStudioReportMessage(ctx context.Context, sourceIds []string, lang string) ([]*schema.Message, error) {
	tmpl := p.newTemplate[StudioReportTemplateVars](templateNameStudioReport, lang)
	msg, err := tmpl.Message(ctx, StudioReportTemplateVars{SourceIds: sourceIds})
	if err != nil {
		return nil, err
	}
	return p.prependSystemMessage([]*schema.Message{msg}), nil
}

// RenderStudioInfoGraphicMessage renders a studio infographic prompt.
func (p *Prompt) RenderStudioInfoGraphicMessage(ctx context.Context, vars StudioInfoGraphicTemplateVars, lang string) ([]*schema.Message, error) {
	tmpl := p.newTemplate[StudioInfoGraphicTemplateVars](templateNameStudioInfographic, lang)
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
	tmpl := p.newTemplate[StudioPodcastOutlineTemplateVars](templateNameStudioPodcastOutline, lang)
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

// newTemplate creates a template from the pre-loaded store.
func (p *Prompt) newTemplate[T templateVars](tmplName templateName, lang string) *template[T] {
	normalizedName := normalizeTemplateName(tmplName)
	normalizedLang := normalizeTemplateLang(lang)
	if normalizedLang == "" {
		normalizedLang = p.defaultLang
	}
	content := readTemplate(normalizedName, normalizedLang)
	return &template[T]{
		name: normalizedName,
		lang: normalizedLang,
		tmpl: fromMessages(schema.Jinja2, schema.UserMessage(content)),
	}
}

func (p *Prompt) prependSystemMessage(msgs []*schema.Message) []*schema.Message {
	return append([]*schema.Message{p.systemMsg}, msgs...)
}
```

Note: `fromMessages` is added to `template.go` in Task 3 as a package-level helper that mirrors the current `prompt.FromMessages` call.

- [ ] **Step 2: Run compilation check**

```bash
go build ./internal/app/biz/prompt/...
```
Expected: Fail — old package-level `Render*` functions still exist in moved files and conflict. This is expected; we remove them in Tasks 3-4.

- [ ] **Step 3: Commit**

```bash
git add internal/app/biz/prompt/prompt.go
git commit -m "feat(prompt): add Prompt struct with render methods"
```

---

### Task 3: Update template.go and system.go — remove globals, add helpers

**Files:**
- Modify: `internal/app/biz/prompt/template.go` — add `fromMessages` helper
- Modify: `internal/app/biz/prompt/system.go` — remove `prependSystemMessage` and `gonotelmSystemPrompt` global

- [ ] **Step 1: Add `fromMessages` helper to template.go**

At the end of `template.go`, add:

```go
func fromMessages(tplType schema.TemplateType, msgs ...*schema.Message) prompt.ChatTemplate {
	return prompt.FromMessages(tplType, msgs...)
}
```

Add the import for `"github.com/cloudwego/eino/schema"` if not already present (it already is).

- [ ] **Step 2: Strip system.go down to just the constant**

Replace `internal/app/biz/prompt/system.go` content:

```go
package prompt

const systemPrompt = "You are GoNoteLM, a powerful intelligent assistant. You will handle specific tasks based on the provided source content."
```

Remove imports for `"github.com/cloudwego/eino/schema"` (no longer needed).

- [ ] **Step 3: Run compilation check**

```bash
go build ./internal/app/biz/prompt/...
```
Expected: Still fails due to old package-level Render* functions. Proceed to Task 4.

- [ ] **Step 4: Commit**

```bash
git add internal/app/biz/prompt/template.go internal/app/biz/prompt/system.go
git commit -m "refactor(prompt): remove globals from template.go and system.go, add fromMessages helper"
```

---

### Task 4: Remove old package-level Render* and New* functions from template files

**Files:**
- Modify: `internal/app/biz/prompt/chat.go` — remove `NewChatTemplate`, `normalizeTemplateLang`
- Modify: `internal/app/biz/prompt/summarize.go` — remove `NewSummarizeTemplate`, `RenderSummarizeMessage`
- Modify: `internal/app/biz/prompt/titlemaker.go` — remove `NewTitleMakerTemplate`, `RenderTitleMakerMessage`
- Modify: `internal/app/biz/prompt/retrievesourcedoc.go` — remove `NewRetrieveSourceDocTemplate`, `RenderRetrieveSourceDocMessage`
- Modify: `internal/app/biz/prompt/notebooksummary.go` — remove `NewNotebookSummaryTemplate`, `RenderNotebookSummaryMessage`
- Modify: `internal/app/biz/prompt/studiomindmap.go` — remove `NewStudioMindmapTemplate`, `RenderStudioMindmapContentMessage`, `RenderStudioMindmapAbstractMessage`, `RenderStudioMindmapMessageWithMode`
- Modify: `internal/app/biz/prompt/studiomindmapv2.go` — remove `NewStudioMindmapV2Template`, `RenderStudioMindmapV2Message`
- Modify: `internal/app/biz/prompt/studioreport.go` — remove `NewStudioReportTemplate`, `RenderStudioReportMessage`
- Modify: `internal/app/biz/prompt/studioinfographic.go` — remove `NewStudioInfoGraphicTemplate`, `RenderStudioInfoGraphicMessage`
- Modify: `internal/app/biz/prompt/studiopodcastoutline.go` — remove `NewStudioPodcastOutlineTemplate`, `RenderStudioPodcastOutlineMessage`

For each file: remove the `New*Template` constructor function and the `Render*` function. Keep all type definitions, type aliases, `*TemplateVars` types, `PromptVars()` methods, constants, and `CheckStudioMindmapResult`.

Example for `summarize.go` — replace the file keeping only types and vars:

```go
package prompt

import (
	"fmt"
	"strings"
)

type SummarizeTemplateVars struct {
	Text    string
	MaxWord int
	MinWord int
}

func (v SummarizeTemplateVars) PromptVars() map[string]any {
	if v.MaxWord <= 0 {
		v.MaxWord = 150
	}
	if v.MinWord <= 0 {
		v.MinWord = 60
	}
	wordRange := fmt.Sprintf("%d-%d", v.MinWord, v.MaxWord)
	if v.MaxWord == v.MinWord {
		wordRange = fmt.Sprintf("%d", v.MinWord)
	}
	return map[string]any{
		"WordRange": wordRange,
		"Text":      strings.TrimSpace(v.Text),
	}
}

type SummarizeTemplate = template[SummarizeTemplateVars]
```

Example for `chat.go` — remove `NewChatTemplate` and `normalizeTemplateLang` (the latter moves to `prompt.go`):

```go
package prompt

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	chatmodel "github.com/gonotelm-lab/gonotelm/internal/app/model/chat"
)

const (
	ChatTemplateStyleDefault = chatmodel.ChatStyleDefault
	ChatTemplateStyleAnalyst = chatmodel.ChatStyleAnalyst
	ChatTemplateStyleGuide   = chatmodel.ChatStyleGuide
)

const (
	ChatTemplateAnswerLengthDefault = chatmodel.ChatAnswerLengthDefault
	ChatTemplateAnswerLengthLonger  = chatmodel.ChatAnswerLengthLonger
	ChatTemplateAnswerLengthShorter = chatmodel.ChatAnswerLengthShorter
)

type ChatTemplateVars struct {
	Notebook        string
	Style           chatmodel.ChatStyle
	AnswerLength    chatmodel.ChatAnswerLength
	SelectedSources []ChatSelectedSourceGroup
}

type ChatSelectedSourceGroup struct {
	SourceIndex int64
	SourceID    string
	Docs        []ChatSelectedSourceDoc
}

type ChatSelectedSourceDoc struct {
	DocIndex int64
	DocID    string
	Content  string
	Score    float32
}

func (v ChatTemplateVars) PromptVars() map[string]any {
	style := string(v.Style)
	if style == "" {
		style = string(ChatTemplateStyleDefault)
	}
	answerLength := string(v.AnswerLength)
	if answerLength == "" {
		answerLength = string(ChatTemplateAnswerLengthDefault)
	}
	return map[string]any{
		"Notebook":        v.Notebook,
		"Style":           style,
		"AnswerLength":    answerLength,
		"SelectedSources": v.SelectedSources,
	}
}

type ChatTemplate = template[ChatTemplateVars]

func normalizeTemplateLang(lang string) string {
	normalizedLang := strings.TrimSpace(lang)
	if normalizedLang == "" {
		normalizedLang = defaultLang
	}
	return normalizedLang
}

// ChatTemplateManager manages chat templates cache and lazy loading.
type ChatTemplateManager struct {
	mu sync.RWMutex
	defaultLang string
	templates   map[string]*ChatTemplate
	loader      func(lang string) (*ChatTemplate, error)
}

func NewChatTemplateManager(defaultLanguage string) (*ChatTemplateManager, error) {
	return newChatTemplateManager(defaultLanguage, newChatTemplate)
}

func newChatTemplateManager(
	defaultLanguage string,
	loader func(lang string) (*ChatTemplate, error),
) (*ChatTemplateManager, error) {
	normalizedLang := normalizeTemplateLang(defaultLanguage)
	if loader == nil {
		return nil, fmt.Errorf("chat template loader is required")
	}
	defaultTemplate, err := loader(normalizedLang)
	if err != nil {
		return nil, fmt.Errorf("init default chat template failed: %w", err)
	}
	return &ChatTemplateManager{
		defaultLang: normalizedLang,
		templates: map[string]*ChatTemplate{
			normalizedLang: defaultTemplate,
		},
		loader: loader,
	}, nil
}

func newChatTemplate(lang string) (*ChatTemplate, error) {
	return newTemplate[ChatTemplateVars](templateNameChat, lang), nil
}

func (m *ChatTemplateManager) Get(lang string) *ChatTemplate {
	normalizedLang := strings.TrimSpace(lang)
	if normalizedLang == "" {
		normalizedLang = m.defaultLang
	}
	m.mu.RLock()
	if tmpl, ok := m.templates[normalizedLang]; ok {
		m.mu.RUnlock()
		return tmpl
	}
	defaultTemplate := m.templates[m.defaultLang]
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if tmpl, ok := m.templates[normalizedLang]; ok {
		return tmpl
	}
	tmpl, err := m.loader(normalizedLang)
	if err != nil {
		slog.Warn("load chat prompt template failed, fallback to default",
			slog.String("lang", normalizedLang),
			slog.Any("err", err),
		)
		return defaultTemplate
	}
	m.templates[normalizedLang] = tmpl
	return tmpl
}
```

Remove the old `NewChatTemplate` function (the public one) and replace `newChatTemplate` with the internal version above. Keep `ChatTemplateManager`, `newChatTemplateManager`, `normalizeTemplateLang`.

Do similar cleanup for each of the 8 remaining files (summarize.go, titlemaker.go, retrievesourcedoc.go, notebooksummary.go, studiomindmap.go, studiomindmapv2.go, studioreport.go, studioinfographic.go, studiopodcastoutline.go).

- [ ] **Step 1: Run compilation check**

```bash
go build ./internal/app/biz/prompt/...
```
Expected: PASS now.

- [ ] **Step 2: Run prompt package tests**

```bash
go test ./internal/app/biz/prompt/...
```
Expected: Tests that reference removed package-level functions will fail. Tests need updating in Task 9.

- [ ] **Step 3: Commit**

```bash
git add internal/app/biz/prompt/
git commit -m "refactor(prompt): remove old package-level Render*/New* functions, keep only types and Prompt struct methods"
```

---

### Task 5: Update logic/chat consumers

**Files:**
- Modify: `internal/app/logic/chat/logic.go` — replace `*prompt.ChatTemplateManager` with `*prompt.Prompt`
- Modify: `internal/app/logic/chat/chatstream.go` — update call to use `l.prompt.ChatTemplate(lang)`

- [ ] **Step 1: Update logic.go**

Change import:
```go
// old
"github.com/gonotelm-lab/gonotelm/internal/app/prompt"
// new
bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
```

Change struct field:
```go
// old
chatTemplateManager *prompt.ChatTemplateManager
// new
prompt *bizprompt.Prompt
```

Change constructor:
```go
func MustNewLogic(
	llmGateway *gateway.Gateway,
	rerankerGateway *rerank.Gateway,
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
	agentSourceBiz *bizsource.AgentBiz,
	chatBiz *bizchat.Biz,
	eventManager *bizchat.ChatEventManager,
	prompt *bizprompt.Prompt,
) *Logic {
	logic := &Logic{
		notebookBiz:         notebookBiz,
		sourceBiz:           sourceBiz,
		chatBiz:             chatBiz,
		eventManager:        eventManager,
		llmGateway:          llmGateway,
		prompt:              prompt,
		sourceDocRetriever:  NewSourceDocRetriever(sourceBiz, agentSourceBiz, llmGateway, rerankerGateway, prompt),
	}
	return logic
}
```

Remove `defaultPromptLang` constant (no longer needed) and `prompt.NewChatTemplateManager(...)` call.

- [ ] **Step 2: Update chatstream.go**

Change `l.chatTemplateManager.Get(state.userLang)` to `l.prompt.ChatTemplate(state.userLang)`:
```go
chatTemplate := l.prompt.ChatTemplate(state.userLang)
```

- [ ] **Step 3: Update utils.go — import path only**

Change import:
```go
// old
"github.com/gonotelm-lab/gonotelm/internal/app/prompt"
// new
bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
```

Update all `prompt.` type references to `bizprompt.`:
- `prompt.ChatTemplateVars` → `bizprompt.ChatTemplateVars`
- `prompt.ChatSelectedSourceGroup` → `bizprompt.ChatSelectedSourceGroup`
- `prompt.ChatSelectedSourceDoc` → `bizprompt.ChatSelectedSourceDoc`

- [ ] **Step 4: Update retrieve.go**

Change import:
```go
// old
"github.com/gonotelm-lab/gonotelm/internal/app/prompt"
// new
bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
```

Update `SourceDocRetriever` struct to accept `*Prompt`:
```go
type SourceDocRetriever struct {
	sourceBiz       *bizsource.Biz
	agentSourceBiz  *bizsource.AgentBiz
	llmGateway      *gateway.Gateway
	rerankerGateway *rerank.Gateway
	prompt          *bizprompt.Prompt
}
```

Update constructor:
```go
func NewSourceDocRetriever(
	sourceBiz *bizsource.Biz,
	agentSourceBiz *bizsource.AgentBiz,
	llmGateway *gateway.Gateway,
	rerankerGateway *rerank.Gateway,
	prompt *bizprompt.Prompt,
) *SourceDocRetriever {
	return &SourceDocRetriever{
		sourceBiz:       sourceBiz,
		agentSourceBiz:  agentSourceBiz,
		llmGateway:      llmGateway,
		rerankerGateway: rerankerGateway,
		prompt:          prompt,
	}
}
```

Update call sites in `agentRetrieve`:
- `prompt.RetrieveSource` → `bizprompt.RetrieveSource`
- `prompt.RenderRetrieveSourceDocMessage(...)` → `s.prompt.RenderRetrieveSourceDocMessage(...)`

- [ ] **Step 5: Commit**

```bash
git add internal/app/logic/chat/
git commit -m "refactor(chat): update logic/chat to use biz/prompt.Prompt struct"
```

---

### Task 6: Update logic/source consumer

**Files:**
- Modify: `internal/app/logic/source/eventhandle.go` — update import and call
- Modify: `internal/app/logic/source/logic.go` — add `*Prompt` field to Logic struct, update constructor

- [ ] **Step 1: Update source/logic.go**

Add import:
```go
bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
```

Add field to `Logic` struct:
```go
type Logic struct {
	// ...existing fields...
	prompt *bizprompt.Prompt
}
```

Add parameter to `MustNewLogic`:
```go
func MustNewLogic(
	rootCtx context.Context,
	infras *infra.Instances,
	objectStorage storage.Storage,
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
	llmGateway *gateway.Gateway,
	prompt *bizprompt.Prompt,
) *Logic {
	sl := &Logic{
		// ...existing fields...
		prompt: prompt,
	}
	// ...rest...
}
```

- [ ] **Step 2: Update source/eventhandle.go**

Change import:
```go
// old
"github.com/gonotelm-lab/gonotelm/internal/app/prompt"
// new
bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
```

Update call in `generateNotebookSummary`:
```go
// old
msgs, err := prompt.RenderNotebookSummaryMessage(ctx, abstracts, pkgcontext.GetLang(ctx))
// new
msgs, err := l.prompt.RenderNotebookSummaryMessage(ctx, abstracts, pkgcontext.GetLang(ctx))
```

- [ ] **Step 3: Commit**

```bash
git add internal/app/logic/source/
git commit -m "refactor(source): update logic/source to use biz/prompt.Prompt struct"
```

---

### Task 7: Update logic/studio consumers

**Files:**
- Modify: `internal/app/logic/studio/logic.go` — add `*Prompt` field and constructor param
- Modify: `internal/app/logic/studio/report.go` — update import and calls
- Modify: `internal/app/logic/studio/mindmap.go` — update import and calls
- Modify: `internal/app/logic/studio/infographic.go` — update import and calls
- Modify: `internal/app/logic/studio/audiooverview.go` — update import and calls

- [ ] **Step 1: Update studio/logic.go**

Add import:
```go
bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
```

Add field:
```go
type Logic struct {
	// ...existing fields...
	prompt *bizprompt.Prompt
}
```

Add param to `MustNewLogic`:
```go
func MustNewLogic(
	ctx context.Context,
	objectStorage storage.Storage,
	sourceBiz *bizsource.Biz,
	sourceBizForAgent *bizsource.AgentBiz,
	notebookBiz *biznotebook.Biz,
	artifactBiz *bizartifact.Biz,
	llmGateway *gateway.Gateway,
	text2imageGateway *text2image.Gateway,
	prompt *bizprompt.Prompt,
) *Logic {
	// ...
	l := &Logic{
		// ...existing fields...
		prompt: prompt,
	}
	// ...
}
```

- [ ] **Step 2: Update studio/report.go**

Change import:
```go
bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
```

Update calls (accessing through `m.l.prompt`):
```go
// old
msgs, err := prompt.RenderStudioReportMessage(ctx, sourceIds, lang)
// new
msgs, err := m.l.prompt.RenderStudioReportMessage(ctx, sourceIds, lang)

// old
titleMakerMsgs, err := prompt.RenderTitleMakerMessage(ctx, report, pkgcontext.GetLang(ctx))
// new
titleMakerMsgs, err := m.l.prompt.RenderTitleMakerMessage(ctx, report, pkgcontext.GetLang(ctx))
```

- [ ] **Step 3: Update studio/mindmap.go**

Change import to `bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"`.

Update calls:
```go
// old
msgs, err = prompt.RenderStudioMindmapAbstractMessage(ctx, tmps, lang)
// new
msgs, err = m.l.prompt.RenderStudioMindmapAbstractMessage(ctx, tmps, lang)

// old
msgs, err = prompt.RenderStudioMindmapContentMessage(ctx, tmps, lang)
// new
msgs, err = m.l.prompt.RenderStudioMindmapContentMessage(ctx, tmps, lang)

// old
msgs, err := prompt.RenderStudioMindmapV2Message(ctx, sourceIds, pkgcontext.GetLang(ctx))
// new
msgs, err := m.l.prompt.RenderStudioMindmapV2Message(ctx, sourceIds, pkgcontext.GetLang(ctx))

// old
if err != nil || !prompt.CheckStudioMindmapResult(expect.Mindmap) {
// new
if err != nil || !bizprompt.CheckStudioMindmapResult(expect.Mindmap) {

// old (line 395)
if !prompt.CheckStudioMindmapResult(expect.Mindmap) {
// new
if !bizprompt.CheckStudioMindmapResult(expect.Mindmap) {
```

- [ ] **Step 4: Update studio/infographic.go**

Change import to `bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"`.

Update calls:
```go
// old
msgs, err := prompt.RenderStudioInfoGraphicMessage(ctx,
    prompt.StudioInfoGraphicTemplateVars{...}, pkgcontext.GetLang(ctx))
// new
msgs, err := m.l.prompt.RenderStudioInfoGraphicMessage(ctx,
    bizprompt.StudioInfoGraphicTemplateVars{...}, pkgcontext.GetLang(ctx))
```

- [ ] **Step 5: Update studio/audiooverview.go**

Change import to `bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"`.

Update calls:
```go
// old
msgs, err := prompt.RenderStudioPodcastOutlineMessage(ctx, sourceIDsToStrings(params.SourceIds),
    params.GetLanguage(), params.GetTip(), prompt.PodcastStyle(params.GetStyle()))
// new
msgs, err := m.l.prompt.RenderStudioPodcastOutlineMessage(ctx, sourceIDsToStrings(params.SourceIds),
    params.GetLanguage(), params.GetTip(), bizprompt.PodcastStyle(params.GetStyle()))
```

- [ ] **Step 6: Commit**

```bash
git add internal/app/logic/studio/
git commit -m "refactor(studio): update logic/studio to use biz/prompt.Prompt struct"
```

---

### Task 8: Update biz/textgen consumers

**Files:**
- Modify: `internal/app/biz/textgen/summarizer/impl.go` — accept `*Prompt`, update method calls
- Modify: `internal/app/biz/textgen/titlemaker/impl.go` — accept `*Prompt`, update method calls

- [ ] **Step 1: Update summarizer/impl.go**

Change import to `bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"`.

Add `prompt` field:
```go
type summazierImpl struct {
	gateway *gateway.Gateway
	option  SummarizeOption
	prompt  *bizprompt.Prompt
}
```

Update constructors:
```go
func New(gateway *gateway.Gateway, prompt *bizprompt.Prompt) Summarizer {
	return NewWithOption(gateway, SummarizeOption{}, prompt)
}

func NewWithOption(gateway *gateway.Gateway, option SummarizeOption, prompt *bizprompt.Prompt) Summarizer {
	return &summazierImpl{
		gateway: gateway,
		option:  option,
		prompt:  prompt,
	}
}
```

Update call:
```go
// old
msgs, err := prompt.RenderSummarizeMessage(ctx, text, lang)
// new
msgs, err := s.prompt.RenderSummarizeMessage(ctx, text, lang)
```

- [ ] **Step 2: Update titlemaker/impl.go**

Same pattern as summarizer — add `prompt` field, update constructors, update call:

Change import to `bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"`.

Add field, update constructors with new param, change `prompt.RenderTitleMakerMessage(...)` to `t.prompt.RenderTitleMakerMessage(...)`.

- [ ] **Step 3: Commit**

```bash
git add internal/app/biz/textgen/
git commit -m "refactor(textgen): update summarizer and titlemaker to use biz/prompt.Prompt struct"
```

---

### Task 9: Update tests

**Files:**
- Modify: `internal/app/biz/prompt/chat_test.go` — update to use `*Prompt`
- Modify: `internal/app/biz/prompt/template_test.go` — update to use `*Prompt`

- [ ] **Step 1: Update chat_test.go**

```go
package prompt

import (
	"errors"
	"testing"
)

func TestNewChatTemplateManagerDefaultLang(t *testing.T) {
	manager, err := newChatTemplateManager("", func(lang string) (*ChatTemplate, error) {
		return &ChatTemplate{lang: lang}, nil
	})
	if err != nil {
		t.Fatalf("new chat template manager failed: %v", err)
	}
	defaultTemplate := manager.Get("")
	if defaultTemplate == nil {
		t.Fatalf("default template should not be nil")
	}
	if defaultTemplate.lang != defaultLang {
		t.Fatalf("unexpected default lang: %s", defaultTemplate.lang)
	}
}

func TestChatTemplateManagerCacheAndFallback(t *testing.T) {
	calls := map[string]int{}
	loader := func(lang string) (*ChatTemplate, error) {
		calls[lang]++
		switch lang {
		case "zh", "en":
			return &ChatTemplate{lang: lang}, nil
		default:
			return nil, errors.New("template not found")
		}
	}
	manager, err := newChatTemplateManager("zh", loader)
	if err != nil {
		t.Fatalf("new chat template manager failed: %v", err)
	}
	defaultTemplate := manager.Get("zh")
	if defaultTemplate == nil {
		t.Fatalf("default template should not be nil")
	}
	enTemplate1 := manager.Get("en")
	enTemplate2 := manager.Get(" en ")
	if enTemplate1 == nil || enTemplate2 == nil {
		t.Fatalf("english template should not be nil")
	}
	if enTemplate1 != enTemplate2 {
		t.Fatalf("english template should be cached")
	}
	if calls["en"] != 1 {
		t.Fatalf("english template should be loaded once, got %d", calls["en"])
	}
	fallbackTemplate1 := manager.Get("fr")
	fallbackTemplate2 := manager.Get("fr")
	if fallbackTemplate1 != defaultTemplate || fallbackTemplate2 != defaultTemplate {
		t.Fatalf("missing lang should fallback to default template")
	}
	if calls["fr"] != 2 {
		t.Fatalf("missing lang should retry loading each time, got %d", calls["fr"])
	}
}

func TestNewChatTemplateManagerWithoutLoader(t *testing.T) {
	_, err := newChatTemplateManager("zh", nil)
	if err == nil {
		t.Fatalf("expected error when loader is nil")
	}
}

func TestPromptNew(t *testing.T) {
	p := New("zh")
	if p == nil {
		t.Fatal("prompt should not be nil")
	}
	if p.defaultLang != "zh" {
		t.Fatalf("unexpected default lang: %s", p.defaultLang)
	}
	tmpl := p.ChatTemplate("zh")
	if tmpl == nil {
		t.Fatal("chat template should not be nil")
	}
}

func TestPromptRenderSummarizeMessage(t *testing.T) {
	p := New("zh")
	msgs, err := p.RenderSummarizeMessage(t.Context(), "测试文本", "zh")
	if err != nil {
		t.Fatalf("render summarize message failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(msgs))
	}
}
```

- [ ] **Step 2: Update template_test.go**

Change all references from package-level `newTemplate` and `NewChatTemplate` to use a `Prompt` instance:

```go
package prompt

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	chatmodel "github.com/gonotelm-lab/gonotelm/internal/app/model/chat"
)

func TestTemplateMessage(t *testing.T) {
	p := New("zh")
	tmpl := p.newTemplate[ChatTemplateVars](templateNameChat, "zh")
	msg, err := tmpl.Message(context.Background(), ChatTemplateVars{
		Notebook: "项目笔记",
		SelectedSources: []ChatSelectedSourceGroup{
			{
				SourceID: "source:1",
				Docs: []ChatSelectedSourceDoc{
					{DocID: "doc:1", Content: "文档片段", Score: 0.98},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("render message failed: %v", err)
	}
	if msg.Role != schema.System {
		t.Fatalf("unexpected role: %s", msg.Role)
	}
	if !strings.Contains(msg.Content, "项目笔记") {
		t.Fatalf("render result does not contain notebook variable")
	}
	if !strings.Contains(msg.Content, "文档片段") {
		t.Fatalf("render result does not contain selected source doc content")
	}
}
```

(Similar updates for all test functions — use `p.newTemplate[...]` instead of package-level `newTemplate[...]`, remove `TestTemplateDefaultLang` which tested the old `NewChatTemplate("")`, and `TestTemplateUnknownLang` which tested `newTemplate[ChatTemplateVars](templateNameChat, "en")` directly — test through `p.newTemplate` instead.)

- [ ] **Step 3: Run tests**

```bash
go test ./internal/app/biz/prompt/... -v
```
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/app/biz/prompt/*_test.go
git commit -m "test(prompt): update tests to use Prompt struct"
```

---

### Task 10: Wire Prompt through central initialization

**Files:**
- Modify: `internal/app/logic/logic.go` — create `*Prompt`, pass to all sub-logics
- Modify: `internal/app/biz/source/sourceindexer.go` — accept `*Prompt`, pass to summarizer
- Modify: `internal/app/biz/source/source.go` — pass `*Prompt` through to `NewSourceIndexer`

- [ ] **Step 1: Update internal/app/logic/logic.go**

Add import:
```go
bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
```

In `MustNewLogic`, after `text2imageGateway` is created, add:
```go
prompt := bizprompt.New("zh")
```

Then update all constructor calls to pass `prompt`:
```go
sourceLogic := sourcelogic.MustNewLogic(
    ctx, infrastructures, objectStorage, notebookBiz, sourceBiz, llmGateway, prompt,
)

chatLogic := chatlogic.MustNewLogic(
    llmGateway, rerankerGateway, notebookBiz, sourceBiz, agentSourceBiz, chatBiz, chatEventManager, prompt,
)

studioLogic := studiologic.MustNewLogic(
    ctx, objectStorage, sourceBiz, agentSourceBiz, notebookBiz, artifactBiz, llmGateway, text2imageGateway, prompt,
)
```

- [ ] **Step 2: Update sourceindexer.go — NewSourceIndexer**

Add import:
```go
bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
```

Add `prompt *bizprompt.Prompt` parameter to `NewSourceIndexer`:
```go
func NewSourceIndexer(
    embedder einoembed.Embedder,
    sourceDocStore vectordal.SourceDocStore,
    objectStorage storage.Storage,
    llmGateway *gateway.Gateway,
    prompt *bizprompt.Prompt,
) *SourceIndexer {
```

Update summarizer creation:
```go
summarizer := summarizer.NewWithOption(
    llmGateway,
    summarizer.SummarizeOption{
        Provider: conf.Global().Logic.Source.ModelProvider,
        Model:    conf.Global().Logic.Source.Model,
    },
    prompt,
)
```

- [ ] **Step 3: Update biz/source/source.go — New()**

Find the `NewSourceIndexer(...)` call and add `prompt` parameter. Accept `*bizprompt.Prompt` in `New()` and thread it through.

- [ ] **Step 4: Run full project compilation**

```bash
go build ./...
```
Expected: Zero errors.

- [ ] **Step 5: Run all tests**

```bash
go test ./... 2>&1 | tail -20
```
Expected: All tests pass (or same results as before refactoring).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: wire Prompt struct through all consumers, fix compilation"
```
