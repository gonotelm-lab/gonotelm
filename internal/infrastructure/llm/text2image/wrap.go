package text2image

import (
	"context"

	pkgtrace "github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/genaiconv"
	"github.com/gonotelm-lab/multimodal/callbacks"
	pkgt2i "github.com/gonotelm-lab/multimodal/image"
	pkgt2ischema "github.com/gonotelm-lab/multimodal/image/schema"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const wrappedGeneratorRunName = "gateway-text2image"

type wrappedGenerator struct {
	provider Text2ImageProvider
	impl     pkgt2i.Generator
}

func newWrappedGenerator(provider Text2ImageProvider, impl pkgt2i.Generator) pkgt2i.Generator {
	return &wrappedGenerator{provider: provider, impl: impl}
}

var _ pkgt2i.Generator = (*wrappedGenerator)(nil)

func (t *wrappedGenerator) Generate(
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
		oteltrace.WithAttributes(genaiconv.Attributes(t.provider.String(), "text2image", model)...),
	)
	defer span.End()

	ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
		Name:      wrappedGeneratorRunName,
		Type:      t.provider.String(),
		Component: callbacks.ComponentImage,
	}, newInterceptor())

	resp, err := t.impl.Generate(ctx, req, opts...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return resp, err
}
