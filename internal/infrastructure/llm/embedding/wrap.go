package embedding

import (
	"context"
	"time"

	pkgtrace "github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/genaiconv"

	embedcache "github.com/cloudwego/eino-ext/components/embedding/cache"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	einoembed "github.com/cloudwego/eino/components/embedding"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	embedCacheGetSpanName = "embedding.cache.get"
	embedCacheSetSpanName = "embedding.cache.set"
)

const (
	wrappedEmbedderName = "gateway-embedder"
)

type wrappedCacher struct {
	impl embedcache.Cacher
}

var _ embedcache.Cacher = (*wrappedCacher)(nil)

func wrapCacher(cacher embedcache.Cacher) embedcache.Cacher {
	if cacher == nil {
		return nil
	}
	return &wrappedCacher{impl: cacher}
}

func (c *wrappedCacher) Get(ctx context.Context, key string) ([]float64, bool, error) {
	ctx, span := pkgtrace.GetOtelTracer().Start(ctx, embedCacheGetSpanName,
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.DBSystemNameRedis,
			semconv.DBOperationName("GET"),
		),
	)
	defer span.End()

	val, hit, err := c.impl.Get(ctx, key)
	span.SetAttributes(attribute.Bool("cache.hit", hit))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return val, hit, err
}

func (c *wrappedCacher) Set(
	ctx context.Context,
	key string,
	value []float64,
	expire time.Duration,
) error {
	ctx, span := pkgtrace.GetOtelTracer().Start(ctx, embedCacheSetSpanName,
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.DBSystemNameRedis,
			semconv.DBOperationName("SET"),
		),
	)
	defer span.End()

	err := c.impl.Set(ctx, key, value, expire)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

type wrappedEmbedder struct {
	system   EmbeddingType
	impl     einoembed.Embedder
	typ      string
	recorder Recorder
}

func newWrappedEmbedder(system EmbeddingType, impl einoembed.Embedder, recorder Recorder) einoembed.Embedder {
	typ, ok := components.GetType(impl)
	if !ok {
		typ = "GatewayEmbedder"
	}

	return &wrappedEmbedder{typ: typ, impl: impl, system: system, recorder: recorder}
}

var _ einoembed.Embedder = (*wrappedEmbedder)(nil)

func (t *wrappedEmbedder) EmbedStrings(
	ctx context.Context,
	texts []string,
	opts ...einoembed.Option,
) ([][]float64, error) {
	model := ""
	if o := einoembed.GetCommonOptions(&einoembed.Options{}, opts...); o != nil && o.Model != nil {
		model = *o.Model
	}

	ctx = withModelName(ctx, model)
	ctx = withProvider(ctx, t.system)

	ctx, span := pkgtrace.GetOtelTracer().
		Start(
			ctx,
			genaiconv.SpanName("embedding"),
			oteltrace.WithSpanKind(oteltrace.SpanKindClient),
			oteltrace.WithAttributes(genaiconv.Attributes(t.system.String(), "embedding", model)...),
		)
	defer span.End()

	ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
		Name:      wrappedEmbedderName,
		Type:      t.typ,
		Component: components.ComponentOfEmbedding,
	}, newInterceptor(t.recorder))

	resp, err := t.impl.EmbedStrings(ctx, texts, opts...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return resp, err
}
