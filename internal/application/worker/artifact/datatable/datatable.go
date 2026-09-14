package datatable

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

const dataTableMaxCompensateRetry = 3

type dataTableExpectation struct {
	Title string `json:"title"`
	Table string `json:"table"`
}

type dataTableGenerator struct {
	deps *types.WorkerDeps
}

func newDataTableGenerator(deps *types.WorkerDeps) *dataTableGenerator {
	return &dataTableGenerator{deps: deps}
}

func (g *dataTableGenerator) generate(
	ctx context.Context,
	req *types.Request,
) (*dataTableExpectation, error) {
	tip := entity.PayloadAs[*entity.DataTablePayload](req.Payload).GetTip()

	msgs, err := RenderDataTable(ctx, types.SourceIDsToStrings(req.SourceIds), tip)
	if err != nil {
		return nil, errors.WithMessagef(err, "generate datatable message failed")
	}

	ag, err := newDataTableAgent(g.deps, req)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*dataTableExpectation](ag, "datatable").
		WithParse(g.parse).
		WithRetry(dataTableMaxCompensateRetry).
		WithRules(dataTableCompensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func dataTableCompensateRules(validateErr error) []string {
	rules := []string{
		"JSON must contain only `title` and `table`",
		"`title` is a single-line title, preferably 10-25 characters",
		"`table` is one GFM Markdown pipe table; newlines inside it must be escaped as \\n",
		"`table` must include a header row, a separator row (---|---), and at least one data row",
		"All rows must have the same column count; no paragraphs/headings/lists outside the table",
	}
	if validateErr != nil {
		rules = append(rules, "Previous failure reason: "+validateErr.Error())
	}
	return rules
}

func (g *dataTableGenerator) parse(ctx context.Context, content string) (*dataTableExpectation, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect dataTableExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx, "datatable direct unmarshal did not match, fallback to json extraction",
				slog.String("err", types.TruncateForLog(err.Error())),
				slog.String("raw_content", types.TruncateForLog(content)))
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "datatable output unmarshal failed after compatibility fallback",
			slog.String("err", types.TruncateForLog(err.Error())),
			slog.String("raw_content", types.TruncateForLog(content)))
		return nil, err
	}

	expect.Title = types.NormalizeTitle(expect.Title)
	table, err := NormalizeDataTableMarkdown(expect.Table)
	if err != nil {
		return nil, err
	}
	if expect.Title == "" {
		return nil, fmt.Errorf("title empty")
	}
	expect.Table = table

	return &expect, nil
}
