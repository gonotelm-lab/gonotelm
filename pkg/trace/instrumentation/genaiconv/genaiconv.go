package genaiconv

import (
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

// genAISystemKey 对应 gen_ai.system 属性（semconv v1.41.0 未内置该 key）。
const genAISystemKey = attribute.Key("gen_ai.system")

// SpanName 返回 gen_ai 操作 span 名，如 "gen_ai.chat"。
func SpanName(operation string) string {
	return "gen_ai." + operation
}

// Attributes 返回 gen_ai client span 属性（gen_ai.operation.name、gen_ai.system、gen_ai.request.model）。
// system 和 model 为空时省略。
func Attributes(system, operation, model string) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		semconv.GenAIOperationNameKey.String(operation),
	}
	if system != "" {
		attrs = append(attrs, genAISystemKey.String(system))
	}
	if model != "" {
		attrs = append(attrs, semconv.GenAIRequestModelKey.String(model))
	}
	return attrs
}
