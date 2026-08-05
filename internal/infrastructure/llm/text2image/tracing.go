package text2image

import (
	"context"

	pkgtrace "github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/genaiconv"
	pkgt2i "github.com/gonotelm-lab/multimodal/image"
	pkgt2ischema "github.com/gonotelm-lab/multimodal/image/schema"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// tracingGenerator 包装 Generator，为每次 Generate 调用创建 gen_ai client span。
type tracingGenerator struct {
	system string
	impl   pkgt2i.Generator
}

var _ pkgt2i.Generator = (*tracingGenerator)(nil)

func (t *tracingGenerator) Generate(
	ctx context.Context,
	req *pkgt2ischema.Request,
	opts ...pkgt2i.Option,
) (*pkgt2ischema.Response, error) {
	model := ""
	if req != nil {
		model = req.Model
	}

	ctx, span := pkgtrace.GetOtelTracer().Start(ctx, genaiconv.SpanName("text2image"),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(genaiconv.Attributes(t.system, "text2image", model)...),
	)
	defer span.End()

	resp, err := t.impl.Generate(ctx, req, opts...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return resp, err
}
