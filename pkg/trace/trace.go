package trace

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	grpcexporter "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	httpexporter "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semver "go.opentelemetry.io/otel/semconv/v1.41.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const TraceName = "gonotelm"

var (
	initOnce      sync.Once
	shutdownOnce  sync.Once
	traceProvider *sdktrace.TracerProvider
	attrResources = make([]attribute.KeyValue, 0)
)

func init() {
	// set propagator
	propagatpor := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagatpor)
}

func Init(ctx context.Context, c Config) error {
	if err := c.normalize(); err != nil {
		return err
	}

	// create exporter
	exporter, err := createExporter(ctx, &c)
	if err != nil {
		slog.ErrorContext(ctx, "[trace] unable to create exporter", slog.Any("err", err))
		return err
	}

	initOnce.Do(func() {
		attrResources = append(attrResources, semver.ServiceNameKey.String(c.Name))

		// set tracer
		traceProvider = sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(c.Sampler))), // 采样率
			sdktrace.WithResource(resource.NewSchemaless(attrResources...)),
			sdktrace.WithBatcher(exporter),
			sdktrace.WithIDGenerator(&IDGenerator{}),
		)

		otel.SetTracerProvider(traceProvider)
		otel.SetErrorHandler(otel.ErrorHandlerFunc(func(cause error) {
			slog.Error(fmt.Sprintf("[otel] error: %v", err))
		}))

		slog.InfoContext(ctx, "[otel] init success")
	})

	return nil
}

func Shutdown(ctx context.Context) {
	shutdownOnce.Do(func() {
		if traceProvider != nil {
			if err := traceProvider.Shutdown(ctx); err != nil {
				slog.ErrorContext(ctx, fmt.Sprintf("[otel] shutdown error: %v", err))
				return
			}

			slog.InfoContext(ctx, "[otel] shutdown done")
		}
	})
}

func createExporter(ctx context.Context, c *Config) (sdktrace.SpanExporter, error) {
	switch c.Exporter {
	case ExporterKindGrpc:
		// create exporters
		return grpcexporter.New(ctx,
			grpcexporter.WithEndpoint(c.Endpoint),
			grpcexporter.WithInsecure(),
		)
	case ExporterKindHttp:
		opts := []httpexporter.Option{
			httpexporter.WithEndpoint(c.Endpoint),
		}
		if len(c.OtlpHttpPath) > 0 {
			opts = append(opts, httpexporter.WithURLPath(c.OtlpHttpPath))
		}
		return httpexporter.New(ctx, opts...)
	}

	return nil, fmt.Errorf("unknown trace exporter: %s", c.Exporter)
}

func GetOtelTracer() oteltrace.Tracer {
	tracer := otel.Tracer(TraceName)
	return tracer
}

func GetTextMapPropagator() propagation.TextMapPropagator {
	return otel.GetTextMapPropagator()
}
