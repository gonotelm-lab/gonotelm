package middleware

import (
	"context"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/requestid"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/grpcconv"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/propagation"

	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpc_codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryTracing 对齐 HTTP Tracing：
// 从 metadata 取 X-Request-Id（没有则生成），Extract trace，开 server span，
// 响应 header 写回 req id 并 Inject traceparent。
func UnaryTracing() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)

		id, err := requestid.ParseString(metadataFirst(md, requestid.HeaderKey))
		if err != nil {
			id = requestid.Gen()
		}

		tracer := trace.GetOtelTracer()
		propagator := trace.GetTextMapPropagator()
		ctx = propagator.Extract(ctx, propagation.NewMetadataCarrier(md))
		ctx = pkgcontext.WithReqId(ctx, id)

		spanName, attrs := grpcconv.ServerAttributes(info.FullMethod)
		ctx, span := tracer.Start(
			ctx,
			spanName,
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			oteltrace.WithAttributes(attrs...),
		)
		defer span.End()

		// 响应 metadata 须在 handler 前写入（与 HTTP 在 Next 前设 header 同理）
		outMD := metadata.MD{}
		outMD.Set(requestid.HeaderKey, id.String())
		propagator.Inject(ctx, propagation.NewMetadataCarrier(outMD))
		_ = grpc.SetHeader(ctx, outMD)

		resp, err := handler(ctx, req)
		grpcconv.ApplyStatus(span, err, grpcStatusCode(err))
		return resp, err
	}
}

func grpcStatusCode(err error) grpc_codes.Code {
	if err == nil {
		return grpc_codes.OK
	}
	if st, ok := status.FromError(err); ok {
		return st.Code()
	}
	return grpc_codes.Unknown
}

func metadataFirst(md metadata.MD, key string) string {
	if md == nil {
		return ""
	}
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
