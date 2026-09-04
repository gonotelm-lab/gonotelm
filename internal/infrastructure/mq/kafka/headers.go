package kafka

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/requestid"
	pkgtrace "github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/propagation"
	"github.com/gonotelm-lab/gonotelm/pkg/ulid"

	"github.com/segmentio/kafka-go"
)

// userIdHeaderKey 是随消息传递的用户 id header。
const userIdHeaderKey = "X-User-Id"

// buildMessageHeaders 注入 trace 上下文及请求上下文（req id / user id）到消息 header。
func buildMessageHeaders(ctx context.Context, headers []mq.MessageHeader) []kafka.Header {
	hds := toKafkaHeaders(headers)
	if reqId := pkgcontext.GetReqId(ctx); !reqId.IsZero() {
		hds = append(hds, kafka.Header{Key: requestid.HeaderKey, Value: []byte(reqId.String())})
	}
	if userId := pkgcontext.GetUserId(ctx); !userId.IsZero() {
		hds = append(hds, kafka.Header{Key: userIdHeaderKey, Value: []byte(userId.String())})
	}

	carrier := propagation.NewKafkaHeaderCarrier(hds)
	pkgtrace.GetTextMapPropagator().Inject(ctx, carrier)

	return carrier.Headers()
}

// restoreRequestContext 从 kafka header 中还原请求上下文（req id / user id）。
func restoreRequestContext(ctx context.Context, headers []kafka.Header) context.Context {
	carrier := propagation.NewKafkaHeaderCarrier(headers)
	if v := carrier.Get(requestid.HeaderKey); v != "" {
		if id, err := requestid.ParseString(v); err == nil {
			ctx = pkgcontext.WithReqId(ctx, id)
		}
	}
	if v := carrier.Get(userIdHeaderKey); v != "" {
		if id, err := ulid.ParseString(v); err == nil {
			ctx = pkgcontext.WithUserId(ctx, id)
		}
	}

	return ctx
}

func toKafkaHeaders(headers []mq.MessageHeader) []kafka.Header {
	if len(headers) == 0 {
		return nil
	}

	hds := make([]kafka.Header, 0, len(headers))
	for _, h := range headers {
		hds = append(hds, kafka.Header{
			Key:   h.Key,
			Value: h.Value,
		})
	}
	return hds
}

func fromKafkaHeaders(headers []kafka.Header) []mq.MessageHeader {
	if len(headers) == 0 {
		return nil
	}

	hds := make([]mq.MessageHeader, 0, len(headers))
	for _, h := range headers {
		hds = append(hds, mq.MessageHeader{
			Key:   h.Key,
			Value: h.Value,
		})
	}
	return hds
}
