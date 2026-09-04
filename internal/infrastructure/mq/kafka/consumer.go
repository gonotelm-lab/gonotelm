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
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgtrace "github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/messagingconv"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/propagation"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

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
		Logger:         kafka.LoggerFunc(kafkaLogger),
		ErrorLogger:    kafka.LoggerFunc(kafkaErrorLogger),
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

// processResult tells the fetch loop what to do after handling a message.
type processResult int

const (
	processContinue processResult = iota // proceed to the next message
	processStop                          // stop the fetch loop
)

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
					return
				}

				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					slog.WarnContext(ctx, "kafka reader ctx done", slog.Any("err", err))
					return
				}

				var kafkaErr kafka.Error
				if stderr.As(err, &kafkaErr) {
					if !kafkaErr.Temporary() {
						slog.ErrorContext(ctx, "kafka fetch got non-temporary kafka error", slog.Any("err", err))
						return
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
					return
				}
				continue
			}

			unknownErrAttempts = 0

			if c.processMessage(ctx, msg, handler) == processStop {
				return
			}
		}
	}()

	return nil
}

const (
	handlerRetryBackoff = 200 * time.Millisecond
	handlerMaxAttempts  = 3
)

func (c *Consumer) processMessage(ctx context.Context, msg kafka.Message, handler mq.MessageHandler) processResult {
	carrier := propagation.NewKafkaHeaderCarrier(msg.Headers)
	propCtx := pkgtrace.GetTextMapPropagator().Extract(ctx, carrier)
	tracer := pkgtrace.GetOtelTracer()
	reqCtx, span := tracer.Start(
		propCtx,
		messagingconv.ProcessSpanName(msg.Topic),
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

	defer func() {
		if rec := recover(); rec != nil {
			err := fmt.Errorf("%v", rec)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slog.ErrorContext(
				ctx,
				"kafka message handler panic",
				slog.Any("err", rec),
				slog.String("stack", string(debug.Stack())),
				slog.String("topic", msg.Topic),
				slog.String("key", string(msg.Key)),
			)
		}
		span.End()
	}()

	reqCtx = restoreRequestContext(reqCtx, msg.Headers)
	kafkaMsg := &KafkaMessage{
		topic:   msg.Topic,
		key:     msg.Key,
		value:   msg.Value,
		headers: msg.Headers,
	}

	var err error
	for attempt := 1; attempt <= handlerMaxAttempts; attempt++ {
		err = handler(reqCtx, kafkaMsg)
		if err == nil {
			break
		}
		span.RecordError(err)

		if attempt == handlerMaxAttempts {
			break
		}

		slog.WarnContext(ctx, "kafka message handler failed, retrying",
			slog.Any("err", err),
			slog.String("topic", msg.Topic),
			slog.String("key", string(msg.Key)),
			slog.Int("attempt", attempt),
		)
		if !waitForBackoffOrDone(ctx, handlerRetryBackoff) {
			slog.WarnContext(ctx, "kafka handler retry interrupted by context", slog.Any("err", ctx.Err()))
			return processStop
		}
	}

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		slog.ErrorContext(ctx, "kafka message handler failed after retries, commit anyway",
			slog.Any("err", err),
			slog.String("topic", msg.Topic),
			slog.String("key", string(msg.Key)),
			slog.String("value", string(msg.Value)),
			slog.Int("attempts", handlerMaxAttempts),
		)
	}

	if cerr := c.r.CommitMessages(ctx, msg); cerr != nil {
		slog.ErrorContext(ctx, "kafka commit messages failed", slog.Any("err", cerr))
	}

	return processContinue
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
