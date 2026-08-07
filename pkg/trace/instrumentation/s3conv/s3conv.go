package s3conv

import (
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

// awsRegionKey 对应 aws.region 属性（otelaws 自定义 key，semconv v1.41.0 未内置）。
const awsRegionKey = attribute.Key("aws.region")

// SpanName 返回 S3 操作 span 名，如 "S3.GetObject"。
func SpanName(operation string) string {
	return "S3." + operation
}

// MethodName 返回 rpc.method 属性值，如 "S3/GetObject"，与 AWS SDK instrumentation 一致。
func MethodName(operation string) string {
	return "S3/" + operation
}

// Attributes 返回 S3 client span 属性，对齐 AWS SDK v2 的 otelaws instrumentation
// （rpc.system.name=aws-api、rpc.method、aws.region、aws.s3.bucket、aws.s3.key）。
// region 和 key 为空时省略。
func Attributes(operation, bucket, region, key string) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		semconv.AWSS3BucketKey.String(bucket),
		semconv.RPCSystemNameKey.String("aws-api"),
		semconv.RPCMethod(MethodName(operation)),
	}
	if region != "" {
		attrs = append(attrs, awsRegionKey.String(region))
	}
	if key != "" {
		attrs = append(attrs, semconv.AWSS3KeyKey.String(key))
	}
	return attrs
}
