// Package attributes 统一存放自定义的 span attribute key。
package attributes

import "go.opentelemetry.io/otel/attribute"

var (
	// S3ObjectCount 表示一次 S3 批量操作的对象数量。
	S3ObjectCount = attribute.Key("s3.object_count")
)
