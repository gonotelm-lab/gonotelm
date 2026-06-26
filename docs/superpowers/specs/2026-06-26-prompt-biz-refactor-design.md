# Refactor internal/app/prompt into biz/prompt Struct

## Summary

Move the `internal/app/prompt` package into `internal/app/biz/prompt`, wrapping all
package-level functions and state into a `Prompt` struct. Update all 10 call sites.

## Architecture

**Before:** `internal/app/prompt/` — package-level globals (`templateStore`,
`gonotelmSystemPrompt`) and functions (e.g. `RenderSummarizeMessage`, `NewChatTemplate`).

**After:** `internal/app/biz/prompt/` — a `Prompt` struct that owns the template store,
system prompt, and chat template manager. All rendering functions become methods.

```
internal/app/biz/prompt/
├── prompt.go          # Prompt struct, New(), all render methods
├── chat.go            # ChatTemplateVars, ChatTemplateManager (moved)
├── template.go        # template[T], preloadedTemplates, template store (moved)
├── system.go          # system prompt (moved)
├── summarize.go       # Summarize template types (moved)
├── titlemaker.go      # TitleMaker template types (moved)
├── retrievesourcedoc.go  # RetrieveSource types (moved)
├── notebooksummary.go    # NotebookSummary types (moved)
├── studiomindmap.go      # StudioMindmap types + CheckStudioMindmapResult (moved)
├── studiomindmapv2.go    # StudioMindmapV2 types (moved)
├── studioreport.go       # StudioReport types (moved)
├── studioinfographic.go  # StudioInfoGraphic types (moved)
├── studiopodcastoutline.go # Podcast types (moved)
├── chat_test.go       # Tests (moved, updated)
├── template_test.go   # Tests (moved, updated)
```

## Prompt Struct

```go
type Prompt struct {
    store       preloadedTemplates
    defaultLang string
    systemMsg   *schema.Message
    chatManager *ChatTemplateManager
}

func New(defaultLang string) *Prompt
```

- `store` is loaded from embedded template files at construction (`.jinja` files).
- `chatManager` is initialized at construction with the default language.
- `systemMsg` is the pre-built `schema.SystemMessage(...)`.

## Methods

All current package-level `Render*` and `New*` functions become methods on `*Prompt`:

| Current function                     | New method on `*Prompt`              |
|--------------------------------------|--------------------------------------|
| `NewChatTemplate(lang)`              | internal, through `chatTemplate(lang)` |
| `RenderSummarizeMessage(ctx,...)`    | `RenderSummarizeMessage(ctx,...)`    |
| `RenderTitleMakerMessage(ctx,...)`   | `RenderTitleMakerMessage(ctx,...)`   |
| `RenderRetrieveSourceDocMessage(...)`| `RenderRetrieveSourceDocMessage(...)`|
| `RenderNotebookSummaryMessage(...)`  | `RenderNotebookSummaryMessage(...)`  |
| `RenderStudioMindmapContentMessage()`| `RenderStudioMindmapContentMessage()`|
| `RenderStudioMindmapAbstractMessage()`| `RenderStudioMindmapAbstractMessage()`|
| `RenderStudioMindmapMessageWithMode()`| `RenderStudioMindmapMessageWithMode()`|
| `RenderStudioMindmapV2Message(...)`  | `RenderStudioMindmapV2Message(...)`  |
| `RenderStudioReportMessage(...)`     | `RenderStudioReportMessage(...)`     |
| `RenderStudioInfoGraphicMessage(...)`| `RenderStudioInfoGraphicMessage(...)`|
| `RenderStudioPodcastOutlineMessage()`| `RenderStudioPodcastOutlineMessage()`|

`CheckStudioMindmapResult(content string) bool` remains a standalone function (no state needed).

`ChatTemplateManager.Get(lang)` → `Prompt.ChatTemplate(lang)` method delegating to the manager.

## Caller Updates (10 files)

Each file changes its import path and receives a `*prompt.Prompt` instance:

| File | Before | After |
|------|--------|-------|
| `logic/chat/logic.go` | creates `prompt.ChatTemplateManager` | receives `*prompt.Prompt`, calls `p.ChatTemplate(lang)` |
| `logic/chat/utils.go` | `prompt.ChatTemplateVars`, etc. | `prompt.ChatTemplateVars` (same type, new package) |
| `logic/chat/retrieve.go` | `prompt.RenderRetrieveSourceDocMessage(...)` | `p.RenderRetrieveSourceDocMessage(...)` |
| `logic/source/eventhandle.go` | `prompt.RenderNotebookSummaryMessage(...)` | `p.RenderNotebookSummaryMessage(...)` |
| `logic/studio/report.go` | `prompt.RenderStudioReportMessage(...)` | `p.RenderStudioReportMessage(...)` |
| `logic/studio/mindmap.go` | `prompt.RenderStudioMindmap*()`, `prompt.CheckStudioMindmapResult()` | `p.RenderStudioMindmap*()`, `prompt.CheckStudioMindmapResult()` |
| `logic/studio/infographic.go` | `prompt.RenderStudioInfoGraphicMessage(...)` | `p.RenderStudioInfoGraphicMessage(...)` |
| `logic/studio/audiooverview.go` | `prompt.RenderStudioPodcastOutlineMessage(...)` | `p.RenderStudioPodcastOutlineMessage(...)` |
| `biz/textgen/titlemaker/impl.go` | `prompt.RenderTitleMakerMessage(...)` | `p.RenderTitleMakerMessage(...)` |
| `biz/textgen/summarizer/impl.go` | `prompt.RenderSummarizeMessage(...)` | `p.RenderSummarizeMessage(...)` |

## DI Strategy

- `*prompt.Prompt` is created once at application startup (in `internal/infra/init.go` or similar).
- Passed to consumers via their constructors or struct fields as needed.
- Replaces the current `ChatTemplateManager` creation in `logic/chat/logic.go`.

## Non-Goals

- No changes to template file content (`.jinja` files remain under `biz/prompt/*/`).
- No changes to the template rendering engine (`template[T]`, `preloadedTemplates`).
- No structural changes to consumer packages beyond import path and DI.

## Test Update

- `chat_test.go` and `template_test.go` move to `biz/prompt/` and update package declaration.
- Test helper `newChatTemplateManager` stays package-private.
