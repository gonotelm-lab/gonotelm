package propagation

import (
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc/metadata"
)

// MetadataCarrier 基于 gRPC metadata 的 TextMapCarrier。
type MetadataCarrier struct {
	md metadata.MD
}

func NewMetadataCarrier(md metadata.MD) *MetadataCarrier {
	if md == nil {
		md = metadata.MD{}
	}
	return &MetadataCarrier{md: md}
}

var _ propagation.TextMapCarrier = (*MetadataCarrier)(nil)

func (c *MetadataCarrier) MD() metadata.MD {
	return c.md
}

func (c *MetadataCarrier) Get(key string) string {
	values := c.md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (c *MetadataCarrier) Set(key, value string) {
	c.md.Set(key, value)
}

func (c *MetadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c.md))
	for k := range c.md {
		keys = append(keys, k)
	}
	return keys
}
