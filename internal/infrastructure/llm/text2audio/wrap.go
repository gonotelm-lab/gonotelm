package text2audio

import (
	"context"

	pkgtrace "github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/genaiconv"
	audios "github.com/gonotelm-lab/multimodal/audio"
	audioschema "github.com/gonotelm-lab/multimodal/audio/schema"
	"github.com/gonotelm-lab/multimodal/callbacks"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const wrappedGeneratorRunName = "gateway-text2audio"

type wrappedGenerator struct {
	provider Text2AudioProvider
	impl     audios.Generator
	recorder Recorder
}

func newWrappedGenerator(provider Text2AudioProvider, impl audios.Generator, recorder Recorder) audios.Generator {
	return &wrappedGenerator{provider: provider, impl: impl, recorder: recorder}
}

var _ audios.Generator = (*wrappedGenerator)(nil)

func (t *wrappedGenerator) Generate(
	ctx context.Context,
	req *audioschema.Request,
	opts ...audios.Option,
) (*audioschema.Response, error) {
	model := ""
	if req != nil {
		model = req.Model
	}

	ctx = withModelName(ctx, model)
	ctx = withProvider(ctx, t.provider)

	ctx, span := pkgtrace.GetOtelTracer().Start(ctx, genaiconv.SpanName("text2audio"),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(genaiconv.Attributes(t.provider.String(), "text2audio", model)...),
	)
	defer span.End()

	ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
		Name:      wrappedGeneratorRunName,
		Type:      t.provider.String(),
		Component: callbacks.ComponentAudio,
	}, newInterceptor(t.recorder))

	resp, err := t.impl.Generate(ctx, req, opts...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return resp, err
}
