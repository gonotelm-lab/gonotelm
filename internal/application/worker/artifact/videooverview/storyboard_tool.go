package videooverview

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	einotool "github.com/cloudwego/eino/components/tool"
	einotoolutils "github.com/cloudwego/eino/components/tool/utils"
)

const (
	writeStoryboardToolName = "WriteStoryboard"
	editStoryboardToolName  = "EditStoryboard"
	readStoryboardToolName  = "ReadStoryboard"
	statStoryboardToolName  = "StatStoryboard"
)

type writeStoryboardInput struct {
	Content string `json:"content" jsonschema:"title=full storyboard markdown,description=The COMPLETE storyboard document: YAML frontmatter (title only) plus contiguous '## Shot 1..N' sections. Replaces any previous draft entirely."`
}

type editStoryboardInput struct {
	OldString  string `json:"old_string"  jsonschema:"title=old_string content,description=The exact existing content to be replaced, must match the file content exactly"`
	NewString  string `json:"new_string"  jsonschema:"title=new_string content,description=The new content to replace the old content with"`
	ReplaceAll bool   `json:"replace_all,omitempty" jsonschema:"title=replace all,description=Replace every occurrence. Default false: the edit fails when old_string matches more than one place."`
}

type readStoryboardInput struct {
	Offset int `json:"offset,omitempty" jsonschema_description:"1-based line number to start reading from. Only provide if the draft is too large to read at once. If omitted or 0, reads from line 1."`
	Limit  int `json:"limit,omitempty"  jsonschema_description:"The number of lines to read. Only provide if the draft is too large to read at once. If omitted or 0, reads to the end."`
}

type statStoryboardInput struct{}

// getStoryboardFileTools 构造 storyboard 步骤的文件工具；三个工具都没有路径参数，
// 只能读写 storyboardFilePath 推导出的那一个文件。
func getStoryboardFileTools(notebookId, artifactId valobj.Id) (map[string]einotool.InvokableTool, error) {
	writeTool, err := einotoolutils.InferTool(
		writeStoryboardToolName,
		"Overwrite the storyboard draft file with the complete document.\n\n"+
			"Usage:\n"+
			"- Put the whole storyboard markdown in `content`: frontmatter (title only) + contiguous `## Shot 1..N`.\n"+
			"- The target file is fixed by the system; you cannot and need not specify a path.\n"+
			"- Calling it again replaces the previous draft entirely; prefer EditStoryboard for a small fix.\n"+
			"- Do not paste the storyboard into the chat reply.",
		func(ctx context.Context, input *writeStoryboardInput) (string, error) {
			content := strings.TrimSpace(input.Content)
			if content == "" {
				return "", fmt.Errorf("content is required")
			}

			written, err := writeStoryboardFile(notebookId, artifactId, content)
			if err != nil {
				return "", err
			}

			slog.DebugContext(ctx, "storyboard tool WriteStoryboard", slog.Int("chars", written))
			return fmt.Sprintf("OK wrote storyboard (%d chars)", written), nil
		},
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "infer %s tool failed: %v", writeStoryboardToolName, err)
	}

	editTool, err := einotoolutils.InferTool(
		editStoryboardToolName,
		"Fix the storyboard draft with an exact string replacement (same principle as EditFile).\n\n"+
			"Usage:\n"+
			"- `old_string` must be copied verbatim from the draft, including indentation and whitespace.\n"+
			"- The edit fails when `old_string` is missing, or matches more than one place and `replace_all` is false — "+
			"then include more surrounding lines to make it unique.\n"+
			"- Use WriteStoryboard instead when the fix touches many shots at once.",
		func(ctx context.Context, input *editStoryboardInput) (string, error) {
			replaced, err := editStoryboardFile(
				notebookId, artifactId, input.OldString, input.NewString, input.ReplaceAll,
			)
			if err != nil {
				return "", err
			}

			slog.DebugContext(ctx, "storyboard tool EditStoryboard", slog.Int("replaced", replaced))
			return fmt.Sprintf("OK edited storyboard (%d replacement(s))", replaced), nil
		},
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "infer %s tool failed: %v", editStoryboardToolName, err)
	}

	readTool, err := einotoolutils.InferTool(
		readStoryboardToolName,
		"Read back the current storyboard draft file by lines.\n\n"+
			"Usage:\n"+
			"- No path argument: the file is fixed by the system.\n"+
			"- By default reads the whole draft from the beginning; use offset (1-based line number) "+
			"and limit to read a long draft in chunks.\n"+
			"- Lines longer than 2000 characters will be truncated.\n"+
			"- Results are returned as 'LINE_NUMBER|LINE_CONTENT', line numbers start at 1.\n"+
			"- Returns `(empty storyboard)` when nothing has been written yet.",
		func(ctx context.Context, input *readStoryboardInput) (string, error) {
			content, err := readStoryboardFile(notebookId, artifactId)
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(content) == "" {
				return "(empty storyboard)", nil
			}

			out := formatStoryboardLines(content, input.Offset, input.Limit)
			if out == "" {
				return "(no lines in that range)", nil
			}

			slog.DebugContext(ctx, "storyboard tool ReadStoryboard",
				slog.Int("offset", input.Offset),
				slog.Int("limit", input.Limit),
				slog.Int("chars", len(out)),
			)
			return out, nil
		},
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "infer %s tool failed: %v", readStoryboardToolName, err)
	}

	statTool, err := einotoolutils.InferTool(
		statStoryboardToolName,
		"Check whether the storyboard draft file exists and how large it is.\n\n"+
			"No arguments and no path: the file is fixed by the system. "+
			"Use it to decide whether to read the whole draft or read it in chunks.",
		func(ctx context.Context, _ *statStoryboardInput) (string, error) {
			size, exists, err := statStoryboardFile(notebookId, artifactId)
			if err != nil {
				return "", err
			}
			if !exists {
				return "(no storyboard file)", nil
			}

			slog.DebugContext(ctx, "storyboard tool StatStoryboard", slog.Int64("bytes", size))
			return fmt.Sprintf("storyboard file exists, %d bytes", size), nil
		},
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "infer %s tool failed: %v", statStoryboardToolName, err)
	}

	return map[string]einotool.InvokableTool{
		writeStoryboardToolName: writeTool,
		editStoryboardToolName:  editTool,
		readStoryboardToolName:  readTool,
		statStoryboardToolName:  statTool,
	}, nil
}
