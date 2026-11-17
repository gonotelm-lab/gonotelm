package grpcconv

import (
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	grpc_codes "google.golang.org/grpc/codes"
)

// ParseFullMethod 按 OTel / otelgrpc 约定解析 `/package.Service/Method`，
// 返回 span 名 `package.Service/Method` 与 rpc.method 属性。
func ParseFullMethod(fullMethod string) (string, []attribute.KeyValue) {
	if !strings.HasPrefix(fullMethod, "/") {
		return fullMethod, nil
	}

	name := fullMethod[1:]
	pos := strings.LastIndex(name, "/")
	if pos < 0 {
		return name, nil
	}

	method := name[pos+1:]
	if method == "" {
		return name, nil
	}
	return name, []attribute.KeyValue{semconv.RPCMethod(method)}
}

// ServerAttributes 返回 gRPC server span 起始属性（rpc.system.name + rpc.method）。
func ServerAttributes(fullMethod string) (spanName string, attrs []attribute.KeyValue) {
	spanName, methodAttrs := ParseFullMethod(fullMethod)
	attrs = append([]attribute.KeyValue{semconv.RPCSystemNameGRPC}, methodAttrs...)
	return spanName, attrs
}

// StatusAttributes 返回结束时的 rpc.response.status_code。
func StatusAttributes(code grpc_codes.Code) attribute.KeyValue {
	return semconv.RPCResponseStatusCode(code.String())
}

// SpanStatus 将 gRPC 错误映射为 span status（server：非 OK 为 Error）。
func SpanStatus(err error) (codes.Code, string) {
	if err == nil {
		return codes.Ok, ""
	}
	return codes.Error, err.Error()
}

// ApplyStatus 写入 span 的 status 与 rpc.response.status_code。
func ApplyStatus(span oteltrace.Span, err error, grpcCode grpc_codes.Code) {
	code, desc := SpanStatus(err)
	span.SetStatus(code, desc)
	span.SetAttributes(StatusAttributes(grpcCode))
	if err != nil {
		span.RecordError(err)
	}
}
