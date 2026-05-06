package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

type StreamState int8

// State 转移
// Init -> [Reasoning] -> [ReasoningEnd] -> [Tooling] -> [Content] -> End
const (
	StreamStateInit         StreamState = iota // 流处理初始化
	StreamStateReasoning                       // 推理中
	StreamStateReasoningEnd                    // 推理结束
	StreamStateTooling                         // 正在接收工具调用
	StreamStateContent                         // 正在接收回复消息
	StreamEnd                                  // 流处理结束
)

type Callbacks struct {
	// OnReasoning 在接收到 reasoning 内容 chunk 时触发（每个 chunk 都回调）。
	OnReasoning func(msg *schema.Message)
	// OnReasoningEnd 在 reasoning 阶段结束时触发（例如切换到 tooling/content）。
	OnReasoningEnd func(msg *schema.Message)
	// OnTooling 在接收到 tool_calls chunk 时触发（每个 chunk 都回调）。
	OnTooling func(msg *schema.Message)
	// OnContent 在接收到正文 content chunk 时触发（每个 chunk 都回调）。
	OnContent func(msg *schema.Message)
	// OnError 在流处理期间发生错误时触发；该回调可能在 OnEnd 之前触发。
	OnError func(err error)
	// OnEnd 在函数退出前触发一次，msg 为最终合并结果（失败时为 nil）。
	OnEnd func(msg *schema.Message)
}

// HandleStreamWithCallback 通过回调处理流式输出，并在当前 goroutine 内同步执行直到流结束。
//
// 行为说明：
//   - 持续调用 stream.Recv() 读取增量消息，并交给内部 tracker 推进状态机。
//   - OnReasoning / OnTooling / OnContent 在接收到对应内容 chunk 时触发（chunk 级回调）。
//   - OnReasoningEnd 在 reasoning 阶段结束时触发（切换到 content/tooling，或 finish_reason/流结束兜底）。
//   - OnEnd 始终在函数退出前触发，参数为最终合并消息；若合并失败则为 nil。
//   - 接收失败、拼接失败、ctx 取消等错误通过 OnError 回调上报。
//
// 参数说明：
//   - ctx: 生命周期控制。ctx.Done() 后中止接收循环，并通过 OnError 上报 ctx.Err()。
//   - stream: 大模型流式输出读取器。该函数不会调用 stream.Close()，由调用方负责关闭。
//   - callbacks: 回调集合。可为 nil；为 nil 时函数仅消费流，不做回调通知。
func HandleStreamWithCallback(
	ctx context.Context,
	stream *schema.StreamReader[*schema.Message],
	callbacks *Callbacks,
) {
	const bufSize = 256
	tracker := newStreamTracker()
	var finalResult *schema.Message
	tmps := make([]*schema.Message, 0, bufSize)
	emitError := func(err error) {
		if callbacks != nil && callbacks.OnError != nil {
			callbacks.OnError(err)
		}
	}
	defer func() {
		if e := recover(); e != nil {
			slog.ErrorContext(ctx, "handle stream panic", slog.Any("err", e))
			emitError(fmt.Errorf("handle stream panic: %v", e))
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
			emitError(fmt.Errorf("receive message failed: %w", err))
			break
		}

		select {
		case <-ctx.Done():
			emitError(ctx.Err())
			break recvLoop
		default:
		}

		tracker.feed(msg, callbacks)

		tmps = append(tmps, msg)
	}

	concat, err := schema.ConcatMessages(tmps)
	if err != nil {
		emitError(fmt.Errorf("concat messages failed: %w", err))
		return
	}
	finalResult = concat
}

type streamTracker struct {
	state        StreamState
	hasReasoning bool
}

func newStreamTracker() *streamTracker {
	return &streamTracker{
		state: StreamStateInit,
	}
}

func (t *streamTracker) feed(msg *schema.Message, callbacks *Callbacks) {
	if msg == nil {
		return
	}

	if msg.ReasoningContent != "" {
		t.hasReasoning = true
		if t.state != StreamStateReasoning {
			t.state = StreamStateReasoning
		}
		if callbacks != nil && callbacks.OnReasoning != nil {
			callbacks.OnReasoning(msg)
		}
	} else if t.state == StreamStateReasoning {
		t.emitReasoningEnd(callbacks, msg)
	}

	if len(msg.ToolCalls) > 0 {
		if t.state == StreamStateReasoning {
			t.emitReasoningEnd(callbacks, msg)
		}
		if t.state != StreamStateTooling {
			t.state = StreamStateTooling
		}
		if callbacks != nil && callbacks.OnTooling != nil {
			callbacks.OnTooling(msg)
		}
	}

	if msg.Content != "" {
		if t.state == StreamStateReasoning {
			t.emitReasoningEnd(callbacks, msg)
		}
		if t.state != StreamStateContent {
			t.state = StreamStateContent
		}
		if callbacks != nil && callbacks.OnContent != nil {
			callbacks.OnContent(msg)
		}
	}

	if hasFinishReason(msg) && t.state == StreamStateReasoning {
		t.emitReasoningEnd(callbacks, msg)
	}
}

func (t *streamTracker) emitReasoningEnd(callbacks *Callbacks, msg *schema.Message) {
	if t.state != StreamStateReasoning {
		return
	}

	t.state = StreamStateReasoningEnd
	if callbacks != nil && callbacks.OnReasoningEnd != nil {
		callbacks.OnReasoningEnd(msg)
	}
}

func (t *streamTracker) finish(callbacks *Callbacks, finalResult *schema.Message) {
	if t.state == StreamStateReasoning {
		t.emitReasoningEnd(callbacks, nil)
	}
	t.state = StreamEnd
	if callbacks != nil && callbacks.OnEnd != nil {
		callbacks.OnEnd(finalResult)
	}
}

func hasFinishReason(msg *schema.Message) bool {
	return msg != nil &&
		msg.ResponseMeta != nil &&
		msg.ResponseMeta.FinishReason != ""
}
