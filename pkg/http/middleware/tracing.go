package middleware

import (
	"context"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/http/adapter"
	"github.com/gonotelm-lab/gonotelm/pkg/requestid"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/httpconv"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/propagation"

	"github.com/cloudwego/hertz/pkg/app"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// 如果请求没有X-Request-Id 则生成
func Tracing(serverName string) app.HandlerFunc {
	return func(ctx context.Context, rc *app.RequestContext) {
		id, err := requestid.ParseString(rc.Request.Header.Get(requestid.HeaderKey))
		if err != nil {
			id = requestid.Gen()
		}

		tracer := trace.GetOtelTracer()
		propagator := trace.GetTextMapPropagator()
		// 从header中提取可能存在的trace
		headerCarrier := propagation.NewHertzRequestHeaderCarrier(&rc.Request.Header)
		ctx = propagator.Extract(ctx, headerCarrier)
		ctx = pkgcontext.WithReqId(ctx, id)

		spanName := string(rc.Request.Method()) + " " + rc.FullPath()
		startOpts := []oteltrace.SpanStartOption{
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		}

		if stdReq, err := adapter.GetCompatRequest(&rc.Request, rc.RemoteAddr().String()); err == nil {
			startOpts = append(startOpts,
				oteltrace.WithAttributes(httpconv.ServerAttributes(serverName, rc.FullPath(), stdReq)...),
			)
		}

		var span oteltrace.Span
		ctx, span = tracer.Start(ctx, spanName, startOpts...)
		defer span.End()

		// 响应头必须在 rc.Next 之前设置并注入 traceparent：SSE 等流式响应在
		// 首个事件写入时就会 flush 响应头，之后设置的 header 将无法到达客户端
		rc.Response.Header.Set(requestid.HeaderKey, id.String())
		rc.Response.Header.Set("Server", "gonotelm")
		propagator.Inject(ctx, propagation.NewHertzResponseHeaderCarrier(&rc.Response.Header))

		rc.Next(ctx)

		// set trace span result
		httpStatusCode := rc.Response.StatusCode()
		span.SetAttributes(httpconv.ResponseAttributes(httpStatusCode)...)
		code, desc := httpconv.SpanStatus(httpStatusCode, oteltrace.SpanKindServer)
		span.SetStatus(code, desc)
	}
}
