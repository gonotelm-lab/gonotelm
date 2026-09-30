package report

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

const reportMaxCompensateRetry = 3

type reportExpectation struct {
	Title  string `json:"title"`
	Report string `json:"report"`
}

// reportStep 生成并解析报告。
type reportStep struct {
	deps *types.WorkerDeps
}

func (s *reportStep) Name() string { return "report" }

func (s *reportStep) Execute(ctx context.Context, data *pipeline.Data) error {
	expect, err := s.generate(ctx, types.RequestFrom(data))
	if err != nil {
		return err
	}
	data.Set(dataKeyResult, expect)
	return nil
}

func (s *reportStep) generate(
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

	ag, err := newReportAgent(s.deps, req)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*reportExpectation](ag, "report").
		WithParse(s.parse).
		WithRetry(reportMaxCompensateRetry).
		WithDuty("Produce the JSON report (title/report) based on the given source content").
		WithRules(reportCompensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func reportCompensateRules(validateErr error) []string {
	rules := []string{
		"JSON must contain only `title` and `report`",
	}
	if validateErr != nil {
		rules = append(rules, "Previous validation error: "+validateErr.Error())
	}
	return rules
}

func (s *reportStep) parse(ctx context.Context, content string) (*reportExpectation, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect reportExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx, "report direct unmarshal did not match, fallback to json extraction",
				slog.String("err", types.TruncateForLog(err.Error())))
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "report output unmarshal failed after compatibility fallback",
			slog.String("err", types.TruncateForLog(err.Error())),
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
