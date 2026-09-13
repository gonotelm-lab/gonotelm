package videooverview

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	einotool "github.com/cloudwego/eino/components/tool"
	einotoolutils "github.com/cloudwego/eino/components/tool/utils"
)

const (
	appendStoryBoardShotToolName = "AppendStoryBoardShot"
	editStoryBoardShotToolName   = "EditStoryBoardShot"
	readStoryBoardShotToolName   = "ReadStoryBoardShot"
	checkStoryboardShotToolName  = "CheckStoryboardShot"
)

type appendStoryBoardShot struct {
	Index   int    `json:"index"   jsonschema:"title=shot index,description=1-based shot order key. Final markdown is sorted by this index."`
	Content string `json:"content" jsonschema:"title=shot content,description=Markdown for this shot (typically one ## Shot …)."`
}

type appendStoryBoardInput struct {
	Shots []appendStoryBoardShot `json:"shots" jsonschema:"title=indexed shots,description=One or more {index, content} shots. Parallel Append is OK because index decides placement."`
}

type editStoryBoardItem struct {
	Index   int    `json:"index"   jsonschema:"title=shot index,description=Which shot to replace or delete"`
	Content string `json:"content" jsonschema:"title=replacement or delete,description=Non-empty: full replacement markdown for that shot. Empty string \"\": DELETE that shot."`
}

type editStoryBoardInput struct {
	Edits []editStoryBoardItem `json:"edits" jsonschema:"title=edits,description=One or more shot edits/deletes. Duplicate index in the same call is rejected."`
}

type readStoryBoardInput struct {
	Offset  int   `json:"offset,omitempty"  jsonschema_description:"1-based position in the sorted shot-index list when indexes is empty. Omit or 0 to start from the first shot."`
	Limit   int   `json:"limit,omitempty"   jsonschema_description:"Number of shots to read when indexes is empty. Omit or 0 to read through the end."`
	Indexes []int `json:"indexes,omitempty" jsonschema_description:"Optional explicit shot indexes to read. When set, offset/limit are ignored."`
}

// checkStoryBoardInput 无必填参数；占位让工具可被无参调用。
type checkStoryBoardInput struct {
	Reason string `json:"reason,omitempty" jsonschema_description:"Optional note for why you are checking (ignored by the tool)."`
}

func (d *storyboardDoc) getBindedTools() (map[string]einotool.InvokableTool, error) {
	appendTool, err := einotoolutils.InferTool(
		appendStoryBoardShotToolName,
		"Append indexed storyboard shots (safe to call in parallel).\n\n"+
			"Usage:\n"+
			"- Each item is {index, content}; index >= 1.\n"+
			"- Prefer one shot per item; index is the ordering key for the final document.\n"+
			"- Parallel AppendStoryBoardShot calls are OK: placement follows index.\n"+
			"- Same index already present is rejected; use EditStoryBoardShot to replace.\n"+
			"- Do not paste the full storyboard into the chat message.",
		func(ctx context.Context, input *appendStoryBoardInput) (string, error) {
			indexes := make([]int, 0, len(input.Shots))
			shots := make([]storyboardShotInput, 0, len(input.Shots))
			for _, s := range input.Shots {
				indexes = append(indexes, s.Index)
				shots = append(shots, storyboardShotInput{Index: s.Index, Content: s.Content})
			}
			slog.DebugContext(ctx, "storyboard tool AppendStoryBoardShot",
				slog.Any("indexes", indexes),
				slog.Int("count", len(indexes)),
			)
			out, err := d.append(shots)
			if err != nil {
				slog.DebugContext(ctx, "storyboard tool AppendStoryBoardShot failed",
					slog.Any("indexes", indexes),
					slog.Any("err", err),
				)
				return "", err
			}
			return out, nil
		},
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "infer %s tool failed: %v", appendStoryBoardShotToolName, err)
	}

	editTool, err := einotoolutils.InferTool(
		editStoryBoardShotToolName,
		"Replace or DELETE storyboard shots by index.\n\n"+
			"Usage:\n"+
			"- edits[].index identifies the shot.\n"+
			"- edits[].content non-empty → replace the whole shot body.\n"+
			"- edits[].content empty string \"\" → DELETE that shot (this is the delete API; there is no separate Delete tool).\n"+
			"  Example delete: {\"edits\":[{\"index\":3,\"content\":\"\"}]}\n"+
			"- Duplicate index in the same call is rejected.",
		func(ctx context.Context, input *editStoryBoardInput) (string, error) {
			indexes := make([]int, 0, len(input.Edits))
			ops := make([]storyboardEditOp, 0, len(input.Edits))
			for _, e := range input.Edits {
				indexes = append(indexes, e.Index)
				ops = append(ops, storyboardEditOp{Index: e.Index, Content: e.Content})
			}
			slog.DebugContext(ctx, "storyboard tool EditStoryBoardShot",
				slog.Any("indexes", indexes),
				slog.Int("count", len(indexes)),
			)
			out, err := d.edit(ops)
			if err != nil {
				slog.DebugContext(ctx, "storyboard tool EditStoryBoardShot failed",
					slog.Any("indexes", indexes),
					slog.Any("err", err),
				)
				return "", err
			}
			return out, nil
		},
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "infer %s tool failed: %v", editStoryBoardShotToolName, err)
	}

	readTool, err := einotoolutils.InferTool(
		readStoryBoardShotToolName,
		"Read storyboard shots by index.\n\n"+
			"Usage:\n"+
			"- Default: offset/limit over shot indexes sorted ascending.\n"+
			"- Or pass indexes for an explicit multi-shot read.",
		func(ctx context.Context, input *readStoryBoardInput) (string, error) {
			slog.DebugContext(ctx, "storyboard tool ReadStoryBoardShot",
				slog.Int("offset", input.Offset),
				slog.Int("limit", input.Limit),
				slog.Any("indexes", input.Indexes),
			)
			out, err := d.read(input.Offset, input.Limit, input.Indexes)
			if err != nil {
				slog.DebugContext(ctx, "storyboard tool ReadStoryBoardShot failed",
					slog.Any("indexes", input.Indexes),
					slog.Any("err", err),
				)
				return "", err
			}
			return out, nil
		},
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "infer %s tool failed: %v", readStoryBoardShotToolName, err)
	}

	checkTool, err := einotoolutils.InferTool(
		checkStoryboardShotToolName,
		"Self-check storyboard continuity before finishing.\n\n"+
			"Reports PASS/FAIL for:\n"+
			"- shot index continuity (must fill 1..max with no gaps)\n"+
			"- audio_id coverage (every narration clip appears exactly once)\n"+
			"- audio_id order (within a shot and across shots must follow the narration track)\n\n"+
			"Call this after bulk Append/Edit. If FAIL, fix with Append/Edit then Check again.\n"+
			"No required arguments.",
		func(ctx context.Context, input *checkStoryBoardInput) (string, error) {
			slog.DebugContext(ctx, "storyboard tool CheckStoryboardShot",
				slog.Int("shots", d.ShotCount()),
				slog.String("reason", input.Reason),
			)
			out := d.checkContinuity()
			slog.DebugContext(ctx, "storyboard tool CheckStoryboardShot result",
				slog.Int("shots", d.ShotCount()),
				slog.Bool("pass", strings.HasPrefix(out, "PASS")),
			)
			return out, nil
		},
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "infer %s tool failed: %v", checkStoryboardShotToolName, err)
	}

	return map[string]einotool.InvokableTool{
		appendStoryBoardShotToolName: appendTool,
		editStoryBoardShotToolName:   editTool,
		readStoryBoardShotToolName:   readTool,
		checkStoryboardShotToolName:  checkTool,
	}, nil
}
