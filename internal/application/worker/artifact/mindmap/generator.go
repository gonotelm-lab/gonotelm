package mindmap

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

const MindmapMaxOnceToken = 32_000

type Generator struct {
	deps *types.WorkerDeps
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{deps: deps}
}

type mindmapExpectation struct {
	Title   string `json:"title"`
	Mindmap string `json:"mindmap"`
}

func (m *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	expect, err := m.generate(ctx, req)
	if err != nil {
		return nil, err
	}

	return &types.Response{
		Title:      expect.Title,
		Result:     pkgstring.AsBytes(expect.Mindmap),
		ResultKind: artifactentity.ResultKindInline,
	}, nil
}

func (m *Generator) generate(
	ctx context.Context,
	req *types.Request,
) (*mindmapExpectation, error) {
	tip := artifactentity.PayloadAs[*artifactentity.MindmapPayload](req.Payload).GetTip()

	msgs, err := RenderMindmap(ctx, types.SourceIDsToStrings(req.SourceIds), tip)
	if err != nil {
		return nil, errors.WithMessagef(err, "generate mindmap message failed")
	}

	ag, err := newMindmapAgent(m.deps, req)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*mindmapExpectation](ag, "mindmap").
		WithParse(m.parseAgentOutput).
		WithRules(mindmapCompensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func mindmapCompensateRules(error) []string {
	return []string{
		"JSON must contain only `title` and `mindmap`",
		"`title` length must be 10-30 characters",
		"`mindmap` must be a complete mermaid mindmap code-block string",
	}
}

func (m *Generator) parseAgentOutput(ctx context.Context, content string) (*mindmapExpectation, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect mindmapExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx, "mindmap direct unmarshal did not match, fallback to json extraction",
				slog.Any("err", err),
				slog.String("raw_content", types.TruncateForLog(content)),
			)
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "mindmap output unmarshal failed after compatibility fallback",
			slog.Any("err", err),
			slog.String("raw_content", types.TruncateForLog(content)))
		return nil, err
	}

	expect.Title = types.NormalizeTitle(expect.Title)
	expect.Mindmap = strings.TrimSpace(expect.Mindmap)

	if !CheckStudioMindmapResult(expect.Mindmap) {
		return nil, fmt.Errorf("mindmap format invalid")
	}

	return &expect, nil
}
