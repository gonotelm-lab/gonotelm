package kafka

import (
	"context"
	stderr "errors"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkglog "github.com/gonotelm-lab/gonotelm/pkg/log"
	"github.com/gonotelm-lab/gonotelm/pkg/requestid"
	"github.com/gonotelm-lab/gonotelm/pkg/ulid"
	pkgtrace "github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/messagingconv"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/propagation"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// userIdHeaderKey 是随消息传递的用户 id header。
const userIdHeaderKey = "X-User-Id"

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

func prepareMessage(req *mq.ProducerSendRequest, headers []kafka.Header) kafka.Message {
	return kafka.Message{
		Topic:   req.Topic,
		Key:     req.Key,
		Value:   req.Value,
		Headers: headers,
	}
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

func (p *Producer) Close(ctx context.Context) error {
	return p.w.Close()
}

type ConsumerConfig struct {
	Brokers        []string
	GroupID        string
	Topic          string
	QueueCapacity  int
	CommitInterval time.Duration
	Username       string
	Password       string
}

type Consumer struct {
	r *kafka.Reader

	groupID string

	mu        sync.RWMutex
	done      chan struct{}
	closeOnce sync.Once
}

var _ mq.Consumer = (*Consumer)(nil)

func NewConsumer(c ConsumerConfig) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        c.Brokers,
		GroupID:        c.GroupID,
		Topic:          c.Topic,
		QueueCapacity:  c.QueueCapacity,
		CommitInterval: c.CommitInterval,
		// Logger:         kafka.LoggerFunc(kafkaLogger), // too many info logs
		ErrorLogger: kafka.LoggerFunc(kafkaErrorLogger),
		Dialer: &kafka.Dialer{
			DualStack: true,
			SASLMechanism: plain.Mechanism{
				Username: c.Username,
				Password: c.Password,
			},
		},
	})
	return &Consumer{r: r, groupID: c.GroupID}
}

func (c *Consumer) Subscribe(ctx context.Context, topic string, handler mq.MessageHandler) error {
	if handler == nil {
		return errors.New("handler is nil")
	}

	c.mu.Lock()
	if c.done != nil {
		c.mu.Unlock()
		return errors.New("consumer already subscribed")
	}
	done := make(chan struct{})
	c.done = done
	c.mu.Unlock()

	go func() {
		defer close(done)
		defer func() {
			if err := recover(); err != nil {
				slog.ErrorContext(
					ctx,
					"kafka reader panic",
					slog.Any("err", err),
					slog.String("stack", string(debug.Stack())),
				)
			}
		}()

		unknownErrAttempts := 0
		for {
			select {
			case <-ctx.Done():
				slog.WarnContext(ctx, "kafka reader ctx done", slog.Any("err", ctx.Err()))
				return
			default:
			}

			msg, err := c.r.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					slog.WarnContext(ctx, "kafka reader is closed", slog.Any("err", err))
					break
				}

				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					slog.WarnContext(ctx, "kafka reader ctx done", slog.Any("err", err))
					break
				}

				var kafkaErr kafka.Error
				if stderr.As(err, &kafkaErr) {
					if !kafkaErr.Temporary() {
						slog.ErrorContext(ctx, "kafka fetch got non-temporary kafka error", slog.Any("err", err))
						break
					}

					unknownErrAttempts = 0
					slog.WarnContext(ctx, "kafka fetch got temporary kafka error, continue", slog.Any("err", err))
					continue
				}

				// Unknown or transient errors should continue, reader will heal internally.
				unknownErrAttempts++
				backoff := fetchUnknownErrBackoff(unknownErrAttempts)
				slog.WarnContext(
					ctx,
					"kafka fetch message failed, retry with backoff",
					slog.Any("err", err),
					slog.Int("attempt", unknownErrAttempts),
					slog.Duration("backoff", backoff),
				)
				if !waitForBackoffOrDone(ctx, backoff) {
					slog.WarnContext(ctx, "kafka fetch backoff interrupted by context", slog.Any("err", ctx.Err()))
					break
				}
				continue
			}

			unknownErrAttempts = 0

			// 从 kafka header 中提取 trace 上下文，并创建 consumer span
			carrier := propagation.NewKafkaHeaderCarrier(msg.Headers)
			propCtx := pkgtrace.GetTextMapPropagator().Extract(ctx, carrier)

			tracer := pkgtrace.GetOtelTracer()
			spanCtx, span := tracer.Start(propCtx, messagingconv.ProcessSpanName(msg.Topic),
				oteltrace.WithSpanKind(oteltrace.SpanKindConsumer),
				oteltrace.WithAttributes(messagingconv.ProcessAttributes(msg.Topic, msg.Key,
					messagingconv.ProcessOptions{
						GroupName: c.groupID,
						Partition: msg.Partition,
						Offset:    msg.Offset,
						Tombstone: len(msg.Value) == 0,
					},
				)...),
			)

			// 从 kafka header 中还原请求上下文
			spanCtx = restoreRequestContext(spanCtx, msg.Headers)

			kafkaMsg := &KafkaMessage{
				topic:   msg.Topic,
				key:     msg.Key,
				value:   msg.Value,
				headers: msg.Headers,
			}
			err = handler(spanCtx, kafkaMsg)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.End()
				slog.ErrorContext(ctx, "kafka message handler failed",
					slog.Any("err", err),
					slog.String("topic", msg.Topic),
					slog.String("key", string(msg.Key)),
					slog.String("value", string(msg.Value)),
				)
				continue
			}
			span.End()

			// handler success, commit offset
			err = c.r.CommitMessages(ctx, msg)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					slog.WarnContext(ctx, "kafka commit canceled by context", slog.Any("err", err))
					break
				}

				slog.ErrorContext(ctx, "kafka commit messages failed", slog.Any("err", err))
			}
		}
	}()

	return nil
}

func (c *Consumer) Close(ctx context.Context) error {
	var closeErr error
	c.closeOnce.Do(func() {
		if err := c.r.Close(); err != nil {
			closeErr = stderr.Join(closeErr, err)
		}

		c.mu.RLock()
		done := c.done
		c.mu.RUnlock()
		if done == nil {
			return
		}

		select {
		case <-done:
		case <-ctx.Done():
			closeErr = stderr.Join(closeErr, ctx.Err())
		}
	})

	return closeErr
}

type KafkaMessage struct {
	topic      string
	key, value []byte
	headers    []kafka.Header
}

var _ mq.Message = (*KafkaMessage)(nil)

func (m *KafkaMessage) Topic() string {
	return m.topic
}

func (m *KafkaMessage) Key() []byte {
	return m.key
}

func (m *KafkaMessage) Value() []byte {
	return m.value
}

func (m *KafkaMessage) Headers() []mq.MessageHeader {
	return fromKafkaHeaders(m.headers)
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

func kafkaLogger(msg string, args ...any) {
	slog.Info(
		formatKafkaLogMsg(msg, args...),
		slog.String(pkglog.AttrKeyComponent, pkglog.ComponentKafkaGo),
	)
}

func kafkaErrorLogger(msg string, args ...any) {
	slog.Error(
		formatKafkaLogMsg(msg, args...),
		slog.String(pkglog.AttrKeyComponent, pkglog.ComponentKafkaGo),
	)
}

func formatKafkaLogMsg(msg string, args ...any) string {
	if len(args) == 0 {
		return msg
	}

	return fmt.Sprintf(msg, args...)
}

func fetchUnknownErrBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// 100ms, 200ms, 400ms... capped at 5s to avoid hot loop.
	delay := 100 * time.Millisecond
	for i := 1; i < attempt; i++ {
		if delay >= 5*time.Second {
			return 5 * time.Second
		}
		delay *= 2
	}
	if delay > 5*time.Second {
		return 5 * time.Second
	}
	return delay
}

func waitForBackoffOrDone(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
