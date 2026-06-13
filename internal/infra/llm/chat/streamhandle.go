package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"
)

type PackedContent struct {
	Content          string
	ReasoningContent string
}

// 处理大模型的流式输出.
// 返回的通道会自动关闭 调用方需要监听此通道关闭 关闭后可以通过 [FinalResult] 字段获取最终结果.
// 流式输出过程中通过Contents返回内容
type HandleStreamResult struct {
	// Content + ReasoningContent
	Contents chan *PackedContent

	// 调用方需要监听此通道关闭 关闭后可以通过 [FinalResult] 字段获取最终结果
	Closed chan struct{}

	// 流式输出合并后的结果
	FinalResult *schema.Message

	// 接收过程中的错误
	Err error
}

// 处理大模型的流式输出
// 该函数不负责调用stream.Close()来关闭流 调用方需要自行调用stream.Close()
func HandleStream(
	ctx context.Context,
	stream *schema.StreamReader[*schema.Message],
) *HandleStreamResult {
	const bufSize = 256
	result := &HandleStreamResult{
		Contents: make(chan *PackedContent, bufSize),
		Closed:   make(chan struct{}),
	}

	tmps := make([]*schema.Message, 0, bufSize)
	close := closeOnce(result.Closed)

	go func() {
		defer func() {
			if e := recover(); e != nil {
				slog.ErrorContext(ctx, "handle stream panic", slog.Any("err", e))
				result.Err = fmt.Errorf("handle stream panic: %v", e)
			}
			close()
		}()

	recvLoop:
		for {
			msg, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}

			if err != nil {
				result.Err = fmt.Errorf("receive message failed: %w", err)
				break
			}

			select {
			case <-ctx.Done():
				result.Err = ctx.Err()
				break recvLoop
			case result.Contents <- &PackedContent{
				Content:          msg.Content,
				ReasoningContent: msg.ReasoningContent,
			}:
			default:
			}

			tmps = append(tmps, msg)
		}

		concat, err := schema.ConcatMessages(tmps)
		if err != nil {
			result.Err = fmt.Errorf("concat messages failed: %w", err)
		} else {
			result.FinalResult = concat
		}
	}()

	return result
}

func closeOnce(ch chan struct{}) func() {
	once := sync.Once{}
	return func() {
		once.Do(func() {
			close(ch)
		})
	}
}

type EventType string

const (
	EventStart          EventType = "start"
	EventContentStart   EventType = "content_start"
	EventContentDelta   EventType = "content_delta"
	EventContentEnd     EventType = "content_end"
	EventToolStart      EventType = "tool_start"
	EventToolDelta      EventType = "tool_delta"
	EventToolEnd        EventType = "tool_end"
	EventReasoningStart EventType = "reasoning_start"
	EventReasoningDelta EventType = "reasoning_delta"
	EventReasoningEnd   EventType = "reasoning_end"
	EventError          EventType = "error"
	EventDone           EventType = "done"
)

type StreamErrorReason string

const (
	StreamErrorReasonPanic             StreamErrorReason = "panic"
	StreamErrorReasonReceiveError      StreamErrorReason = "receive_error"
	StreamErrorReasonContextDone       StreamErrorReason = "context_done"
	StreamErrorReasonConcatError       StreamErrorReason = "concat_error"
	StreamErrorReasonModelFinishReason StreamErrorReason = "model_finish_reason_error"
	StreamErrorReasonUnknown           StreamErrorReason = "unknown_error"
)

type StreamError struct {
	Reason  StreamErrorReason
	Message string
}

func (e *StreamError) Error() string {
	if e == nil {
		return "stream error"
	}
	if e.Reason != "" && e.Message != "" {
		return fmt.Sprintf("%s: %s", string(e.Reason), e.Message)
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Reason != "" {
		return string(e.Reason)
	}
	return "stream error"
}

type Callbacks struct {
	OnStart func()

	OnContentStart func()
	OnContentDelta func(delta string)
	OnContentEnd   func()

	OnToolStart func()
	OnToolDelta func(delta []schema.ToolCall)
	OnToolEnd   func()

	OnReasoningStart func()
	OnReasoningDelta func(delta string)
	OnReasoningEnd   func()

	OnError func(err error)
	OnDone  func(msg *schema.Message)
}

// HandleStreamWithCallback 读取 LLM 流并按阶段触发回调。
// 使用方式：
//  1. 传入可取消的 ctx 和 stream；函数会阻塞直到 EOF、错误或 ctx 取消。
//  2. 按需实现 callbacks 中的 On* 回调；未设置的回调会被安全跳过。
//  3. 正常结束时触发 OnDone(聚合后的完整消息)，出错时仅触发一次 OnError。
func HandleStreamWithCallback(
	ctx context.Context,
	stream *schema.StreamReader[*schema.Message],
	callbacks *Callbacks,
) {
	const bufSize = 256
	tracker := newStreamTracker()
	hasError := false
	var finalResult *schema.Message
	tmps := make([]*schema.Message, 0, bufSize)

	emitError := func(streamErr *StreamError) {
		if streamErr == nil || hasError {
			return
		}
		hasError = true
		if callbacks != nil && callbacks.OnError != nil {
			callbacks.OnError(streamErr)
		}
	}
	emitStdError := func(reason StreamErrorReason, err error) {
		if err == nil {
			return
		}
		if reason == "" {
			reason = StreamErrorReasonUnknown
		}
		emitError(&StreamError{
			Reason:  reason,
			Message: err.Error(),
		})
	}

	defer func() {
		if e := recover(); e != nil {
			slog.ErrorContext(ctx, "handle stream panic", slog.Any("err", e))
			emitError(&StreamError{
				Reason:  StreamErrorReasonPanic,
				Message: fmt.Sprintf("handle stream panic: %v", e),
			})
		}
		if hasError {
			return
		}
		tracker.finish(callbacks, finalResult)
	}()

recvLoop:
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			emitStdError(StreamErrorReasonReceiveError, fmt.Errorf("receive message failed: %w", err))
			break
		}

		select {
		case <-ctx.Done():
			emitStdError(StreamErrorReasonContextDone, ctx.Err())
			break recvLoop
		default:
		}

		finishReason := msgFinishReason(msg)
		if finishReason != "" && !isNonErrorFinishReason(finishReason) {
			emitError(&StreamError{
				Reason:  StreamErrorReasonModelFinishReason,
				Message: fmt.Sprintf("model returned error finish_reason: %s", finishReason),
			})
			break recvLoop
		}

		tracker.feed(msg, callbacks)

		tmps = append(tmps, msg)
	}

	if hasError {
		return
	}

	concat, err := schema.ConcatMessages(tmps)
	if err != nil {
		emitStdError(StreamErrorReasonConcatError, fmt.Errorf("concat messages failed: %w", err))
		return
	}
	finalResult = concat
}

type streamTracker struct {
	started bool
	lastMsg *schema.Message // 上一个消息
}

func newStreamTracker() *streamTracker {
	return &streamTracker{}
}

func (t *streamTracker) feed(curMsg *schema.Message, callbacks *Callbacks) {
	if curMsg != nil && !t.started {
		t.started = true
		if callbacks != nil && callbacks.OnStart != nil {
			callbacks.OnStart()
		}
	}

	t.emitByTransition(t.lastMsg, curMsg, callbacks)
	t.lastMsg = curMsg
}

func (t *streamTracker) finish(
	callbacks *Callbacks,
	finalResult *schema.Message,
) {
	t.emitByTransition(t.lastMsg, nil, callbacks)
	t.lastMsg = nil

	if callbacks != nil && callbacks.OnDone != nil {
		callbacks.OnDone(finalResult)
	}
}

func (t *streamTracker) emitByTransition(
	prevMsg *schema.Message,
	curMsg *schema.Message,
	callbacks *Callbacks,
) {
	prevHasReasoning := hasReasoningDelta(prevMsg)
	prevHasContent := hasContentDelta(prevMsg)
	prevHasTool := hasToolDelta(prevMsg)

	curHasReasoning := hasReasoningDelta(curMsg)
	curHasContent := hasContentDelta(curMsg)
	curHasTool := hasToolDelta(curMsg)

	// 先发 end：上一块有、当前块没有，即认为结束。
	if prevHasReasoning && !curHasReasoning {
		if callbacks != nil && callbacks.OnReasoningEnd != nil {
			callbacks.OnReasoningEnd()
		}
	}
	if prevHasContent && !curHasContent {
		if callbacks != nil && callbacks.OnContentEnd != nil {
			callbacks.OnContentEnd()
		}
	}
	if prevHasTool && !curHasTool {
		if callbacks != nil && callbacks.OnToolEnd != nil {
			callbacks.OnToolEnd()
		}
	}

	// 再发 start：上一块没有、当前块有，即认为开始。
	if !prevHasReasoning && curHasReasoning {
		if callbacks != nil && callbacks.OnReasoningStart != nil {
			callbacks.OnReasoningStart()
		}
	}
	if !prevHasContent && curHasContent {
		if callbacks != nil && callbacks.OnContentStart != nil {
			callbacks.OnContentStart()
		}
	}
	if !prevHasTool && curHasTool {
		if callbacks != nil && callbacks.OnToolStart != nil {
			callbacks.OnToolStart()
		}
	}

	// 最后发 delta：当前块有增量就触发。
	if curHasReasoning && callbacks != nil && callbacks.OnReasoningDelta != nil {
		callbacks.OnReasoningDelta(curMsg.ReasoningContent)
	}
	if curHasContent && callbacks != nil && callbacks.OnContentDelta != nil {
		callbacks.OnContentDelta(curMsg.Content)
	}
	if curHasTool && callbacks != nil && callbacks.OnToolDelta != nil {
		callbacks.OnToolDelta(curMsg.ToolCalls)
	}
}

func hasReasoningDelta(msg *schema.Message) bool {
	return msg != nil && msg.ReasoningContent != ""
}

func hasContentDelta(msg *schema.Message) bool {
	return msg != nil && msg.Content != ""
}

func hasToolDelta(msg *schema.Message) bool {
	return msg != nil && len(msg.ToolCalls) > 0
}

func msgFinishReason(msg *schema.Message) string {
	if msg == nil || msg.ResponseMeta == nil {
		return ""
	}
	return strings.TrimSpace(msg.ResponseMeta.FinishReason)
}

func isNonErrorFinishReason(reason string) bool {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case FinishReasonStop, FinishReasonLength, FinishReasonToolCalls:
		return true
	default:
		return false
	}
}
