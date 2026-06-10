package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestHandleStreamWithCallbackV2_RecoversPanic(t *testing.T) {
	stream := schema.StreamReaderFromArray([]*schema.Message{
		{
			Role:    schema.Assistant,
			Content: "hello",
		},
	})
	defer stream.Close()

	var gotErr error
	doneCalled := false
	contentEndCalled := false
	HandleStreamWithCallbackV2(context.Background(), stream, &CallbacksV2{
		OnContentDelta: func(_ string) {
			panic("boom")
		},
		OnContentEnd: func() {
			contentEndCalled = true
		},
		OnError: func(err error) {
			gotErr = err
		},
		OnDone: func(msg *schema.Message) {
			doneCalled = true
			if msg != nil {
				t.Fatalf("done msg should be nil on panic, got=%+v", msg)
			}
		},
	})

	if gotErr == nil {
		t.Fatal("expected OnError to be called on panic")
	}
	streamErr, ok := gotErr.(*StreamError)
	if !ok {
		t.Fatalf("expected StreamError, got=%T", gotErr)
	}
	if streamErr.Reason != StreamErrorReasonPanic {
		t.Fatalf("unexpected stop reason, got=%q want=%q", streamErr.Reason, StreamErrorReasonPanic)
	}
	if doneCalled {
		t.Fatal("OnDone should not be called when OnError is triggered")
	}
	if contentEndCalled {
		t.Fatal("content end should not be called when OnError is triggered")
	}
}

func TestHandleStreamWithCallbackV2_FinishReasonClassification(t *testing.T) {
	tests := []struct {
		name          string
		reason        string
		wantError     bool
		wantOnDone    bool
		wantErrReason StreamErrorReason
	}{
		{name: "stop", reason: FinishReasonStop, wantError: false, wantOnDone: true},
		{name: "length", reason: FinishReasonLength, wantError: false, wantOnDone: true},
		{name: "tool_calls", reason: FinishReasonToolCalls, wantError: true, wantOnDone: false, wantErrReason: StreamErrorReasonModelFinishReason},
		{name: "content_filter", reason: FinishReasonContentFilter, wantError: true, wantOnDone: false, wantErrReason: StreamErrorReasonModelFinishReason},
		{name: "unknown", reason: "unknown_reason", wantError: true, wantOnDone: false, wantErrReason: StreamErrorReasonModelFinishReason},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			stream := schema.StreamReaderFromArray([]*schema.Message{
				{
					Role:    schema.Assistant,
					Content: "hello",
					ResponseMeta: &schema.ResponseMeta{
						FinishReason: tt.reason,
					},
				},
			})
			defer stream.Close()

			var gotErr error
			doneCalled := false
			HandleStreamWithCallbackV2(context.Background(), stream, &CallbacksV2{
				OnError: func(err error) {
					gotErr = err
				},
				OnDone: func(_ *schema.Message) {
					doneCalled = true
				},
			})

			if tt.wantError {
				if gotErr == nil {
					t.Fatalf("expected error for reason=%q", tt.reason)
				}
				streamErr, ok := gotErr.(*StreamError)
				if !ok {
					t.Fatalf("expected StreamError, got=%T", gotErr)
				}
				if streamErr.Reason != tt.wantErrReason {
					t.Fatalf("unexpected stop reason, got=%q want=%q", streamErr.Reason, tt.wantErrReason)
				}
				if !strings.Contains(streamErr.Message, tt.reason) {
					t.Fatalf("expected error message to contain finish reason, got=%q want_contains=%q", streamErr.Message, tt.reason)
				}
			} else if gotErr != nil {
				t.Fatalf("did not expect error for reason=%q, got=%v", tt.reason, gotErr)
			}

			if doneCalled != tt.wantOnDone {
				t.Fatalf("OnDone mismatch for reason=%q, got=%v want=%v", tt.reason, doneCalled, tt.wantOnDone)
			}
		})
	}
}

func TestHandleStreamWithCallbackV2_SuccessCallsDoneOnly(t *testing.T) {
	stream := schema.StreamReaderFromArray([]*schema.Message{
		{
			Role:    schema.Assistant,
			Content: "hello",
		},
	})
	defer stream.Close()

	var gotErr error
	doneCalled := false
	HandleStreamWithCallbackV2(context.Background(), stream, &CallbacksV2{
		OnError: func(err error) {
			gotErr = err
		},
		OnDone: func(msg *schema.Message) {
			doneCalled = true
			if msg == nil {
				t.Fatal("done message should not be nil in success path")
			}
			if msg.Content != "hello" {
				t.Fatalf("unexpected done content, got=%q want=%q", msg.Content, "hello")
			}
		},
	})

	if gotErr != nil {
		t.Fatalf("OnError should not be called in success path: %v", gotErr)
	}
	if !doneCalled {
		t.Fatal("expected OnDone to be called in success path")
	}
}

func TestModelWithCallbackV2(t *testing.T) {
	modelWithTools := mustNewTestModelWithTools(t)
	messages := buildTestMessages("当你你在输出思考内容的时候，使用一句话来描述你正在思考什么")

	const maxRound = 10
	for round := 0; round < maxRound; round++ {
		if round > 0 {
			fmt.Println("=======")
		}

		callbackStream, err := modelWithTools.Stream(
			t.Context(), messages, einomodel.WithMaxTokens(1000),
		)
		if err != nil {
			t.Fatal(err)
		}

		var endMsg *schema.Message
		var hasError bool
		HandleStreamWithCallbackV2(t.Context(), callbackStream, &CallbacksV2{
			OnStart: func() {
				fmt.Println("[callback-v2] start")
			},
			OnReasoningStart: func() {
				fmt.Println("[callback-v2] reasoning start")
			},
			OnReasoningDelta: func(delta string) {
				fmt.Printf("[callback-v2] reasoning delta=%s\n", delta)
			},
			OnReasoningEnd: func() {
				fmt.Println("[callback-v2] reasoning end")
			},
			OnContentStart: func() {
				fmt.Println("[callback-v2] content start")
			},
			OnContentDelta: func(delta string) {
				fmt.Printf("[callback-v2] content delta=%s\n", delta)
			},
			OnContentEnd: func() {
				fmt.Println("[callback-v2] content end")
			},
			OnToolStart: func() {
				fmt.Println("[callback-v2] tool start")
			},
			OnToolDelta: func(delta []schema.ToolCall) {
				bb, _ := json.Marshal(delta)
				fmt.Printf("[callback-v2] tool delta=%v\n", string(bb))
			},
			OnToolEnd: func() {
				fmt.Println("[callback-v2] tool end")
			},
			OnError: func(err error) {
				hasError = true
				t.Errorf("callback-v2 stream failed: %v", err)
			},
			OnDone: func(msg *schema.Message) {
				if msg == nil {
					t.Error("callback-v2 stream final result is nil")
					return
				}
				endMsg = msg
				fmt.Printf("[callback-v2] done tooling total=%d\n", len(msg.ToolCalls))
				fmt.Printf("[callback-v2] done content=%s\n", msg.Content)
				fmt.Printf("[callback-v2] done reasoning=%s\n", msg.ReasoningContent)
				jb, _ := json.MarshalIndent(msg, " ", " ")
				fmt.Println(string(jb))
			},
		})
		callbackStream.Close()

		if hasError {
			t.Fatal("callback-v2 stream has error")
		}
		if endMsg == nil {
			t.Fatal("callback-v2 stream end message is nil")
		}

		messages = append(messages, endMsg)
		if len(endMsg.ToolCalls) == 0 {
			return
		}

		for _, tc := range endMsg.ToolCalls {
			toolResult, err := runFakeTool(tc)
			if err != nil {
				t.Fatalf("run fake tool failed, tool=%s err=%v", tc.Function.Name, err)
			}

			messages = append(messages, &schema.Message{
				Role:       schema.Tool,
				ToolCallID: tc.ID,
				ToolName:   tc.Function.Name,
				Content:    toolResult,
			})
		}
	}

	t.Fatalf("chat round exceeded max rounds=%d", maxRound)
}
