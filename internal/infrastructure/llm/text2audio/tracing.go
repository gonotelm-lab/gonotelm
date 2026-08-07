package text2audio

import (
	"context"

	pkgtrace "github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/genaiconv"
	audios "github.com/gonotelm-lab/multimodal/audio"
	audioschema "github.com/gonotelm-lab/multimodal/audio/schema"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// tracingGenerator 包装 Generator，为每次 Generate 调用创建 gen_ai client span。
type tracingGenerator struct {
	system string
	impl   audios.Generator
}

var _ audios.Generator = (*tracingGenerator)(nil)

func (t *tracingGenerator) Generate(
	ctx context.Context,
	req *audioschema.Request,
	opts ...audios.Option,
) (*audioschema.Response, error) {
	model := ""
	if req != nil {
		model = req.Model
	}

	ctx, span := pkgtrace.GetOtelTracer().Start(ctx, genaiconv.SpanName("text2audio"),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(genaiconv.Attributes(t.system, "text2audio", model)...),
	)
	defer span.End()

	resp, err := t.impl.Generate(ctx, req, opts...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return resp, err
}
