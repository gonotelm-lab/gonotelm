package kafka

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgtrace "github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/messagingconv"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type ProducerConfig struct {
	Brokers  []string
	Username string
	Password string
}

type messageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type Producer struct {
	w messageWriter
}

var _ mq.Producer = (*Producer)(nil)

func NewProducer(c ProducerConfig) *Producer {
	transport := &kafka.Transport{
		SASL: plain.Mechanism{
			Username: c.Username,
			Password: c.Password,
		},
	}
	w := &kafka.Writer{
		Addr:         kafka.TCP(c.Brokers...),
		RequiredAcks: kafka.RequireOne,
		Logger:       kafka.LoggerFunc(kafkaLogger),
		ErrorLogger:  kafka.LoggerFunc(kafkaErrorLogger),
		Transport:    transport,
	}
	return &Producer{w: w}
}

func (p *Producer) Send(ctx context.Context, req *mq.ProducerSendRequest) error {
	tracer := pkgtrace.GetOtelTracer()
	ctx, span := tracer.Start(ctx,
		messagingconv.SendSpanName(req.Topic),
		oteltrace.WithSpanKind(oteltrace.SpanKindProducer),
		oteltrace.WithAttributes(messagingconv.SendAttributes(req.Topic, req.Key,
			messagingconv.SendOptions{Tombstone: len(req.Value) == 0},
		)...),
	)
	defer span.End()

	hds := buildMessageHeaders(ctx, req.Headers)

	msg := prepareMessage(req, hds)

	err := p.w.WriteMessages(ctx, msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return errors.Wrap(err, "write messages failed")
	}

	return nil
}

func (p *Producer) Close(ctx context.Context) error {
	return p.w.Close()
}

func prepareMessage(req *mq.ProducerSendRequest, headers []kafka.Header) kafka.Message {
	return kafka.Message{
		Topic:   req.Topic,
		Key:     req.Key,
		Value:   req.Value,
		Headers: headers,
	}
}
