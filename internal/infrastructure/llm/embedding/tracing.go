package embedding

import (
	"context"

	einoembed "github.com/cloudwego/eino/components/embedding"
	pkgtrace "github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/genaiconv"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// tracingEmbedder 包装 Embedder，为每次 EmbedTexts 调用创建 gen_ai client span。
type tracingEmbedder struct {
	system string
	impl   einoembed.Embedder
}

var _ einoembed.Embedder = (*tracingEmbedder)(nil)

func (t *tracingEmbedder) EmbedStrings(
	ctx context.Context,
	texts []string,
	opts ...einoembed.Option,
) ([][]float64, error) {
	model := ""
	if o := einoembed.GetCommonOptions(&einoembed.Options{}, opts...); o != nil && o.Model != nil {
		model = *o.Model
	}

	ctx, span := pkgtrace.GetOtelTracer().Start(ctx, genaiconv.SpanName("embedding"),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(genaiconv.Attributes(t.system, "embedding", model)...),
	)
	defer span.End()

	resp, err := t.impl.EmbedStrings(ctx, texts, opts...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return resp, err
}
