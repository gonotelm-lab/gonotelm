package report

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

const reportMaxCompensateRetry = 3

type Generator struct {
	deps *types.WorkerDeps
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{deps: deps}
}

type reportExpectation struct {
	Title  string `json:"title"`
	Report string `json:"report"`
}

func (r *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	expect, err := r.generate(ctx, req)
	if err != nil {
		return nil, err
	}

	return &types.Response{
		Title:      expect.Title,
		Result:     pkgstring.AsBytes(expect.Report),
		ResultKind: artifactentity.ResultKindInline,
	}, nil
}

func (r *Generator) generate(
	ctx context.Context,
	req *types.Request,
) (*reportExpectation, error) {
	style := artifactentity.ReportStyleDefaultStyle()

	p := artifactentity.PayloadAs[*artifactentity.ReportPayload](req.Payload)
	if p.Style.Supported() {
		style = p.Style
	}

	msgs, err := RenderReport(ctx, types.SourceIDsToStrings(req.SourceIds), style, p.Language, p.GetTip())
	if err != nil {
		return nil, errors.WithMessagef(err, "generate report message failed")
	}

	ag, err := newReportAgent(r.deps, req)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*reportExpectation](ag, "report").
		WithParse(parseAgentOutput).
		WithRetry(reportMaxCompensateRetry).
		WithRules(reportCompensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func reportCompensateRules(validateErr error) []string {
	rules := []string{
		"JSON must contain only `title` and `report`",
		"`title` is a single-line title, preferably 10-25 characters",
		"`report` is the Markdown report body; newlines inside it must be escaped as \\n",
	}
	if validateErr != nil {
		rules = append(rules, "Previous validation error: "+validateErr.Error())
	}
	return rules
}

func parseAgentOutput(ctx context.Context, content string) (*reportExpectation, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect reportExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx, "report direct unmarshal did not match, fallback to json extraction",
				slog.Any("err", err),
				slog.String("raw_content", types.TruncateForLog(content)))
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "report output unmarshal failed after compatibility fallback",
			slog.Any("err", err),
			slog.String("raw_content", types.TruncateForLog(content)))
		return nil, err
	}

	expect.Title = types.NormalizeTitle(expect.Title)
	expect.Report = strings.TrimSpace(expect.Report)

	if expect.Title == "" {
		return nil, fmt.Errorf("title empty")
	}
	if expect.Report == "" {
		return nil, fmt.Errorf("report body empty")
	}

	return &expect, nil
}
