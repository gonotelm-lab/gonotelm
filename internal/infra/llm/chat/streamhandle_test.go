package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func buildTestMessages(extraSystemPrompt string) []*schema.Message {
	systemPrompt := "你是一个会调用工具的助手。你必须优先调用 fake_weather 和 fake_stock，同时调用" +
		"拿到工具结果后再给最终结论。"
	if extraSystemPrompt != "" {
		systemPrompt += extraSystemPrompt
	}

	return []*schema.Message{
		{
			Role:    schema.System,
			Content: systemPrompt,
		},
		{
			Role:    schema.User,
			Content: "请查询北京天气和TSLA价格，然后给一段总结。",
		},
	}
}

func mustNewTestModelWithTools(t *testing.T) einomodel.ToolCallingChatModel {
	t.Helper()

	model, err := New(t.Context(), Openai, &ProviderConfig{
		Openai: OpenaiConfig{
			ApiKey:  os.Getenv("ENV_GONOTELM_OPENAI_API_KEY"),
			BaseUrl: os.Getenv("ENV_GONOTELM_OPENAI_BASE_URL"),
			Model:   os.Getenv("ENV_GONOTELM_OPENAI_MODEL"),
		},
	})
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	modelWithTools, err := model.WithTools([]*schema.ToolInfo{
		{
			Name: "fake_weather",
			Desc: "Query mock weather by city.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"city": {Type: schema.String, Desc: "city name", Required: true},
			}),
		},
		{
			Name: "fake_stock",
			Desc: "Query mock stock price by symbol.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"symbol": {Type: schema.String, Desc: "stock symbol", Required: true},
			}),
		},
	})
	if err != nil {
		t.Fatalf("failed to bind tools: %v", err)
	}

	return modelWithTools
}

func runFakeTool(tc schema.ToolCall) (string, error) {
	switch tc.Function.Name {
	case "fake_weather":
		var in struct {
			City string `json:"city"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &in); err != nil {
			return "", fmt.Errorf("decode fake_weather args failed: %w", err)
		}
		if in.City == "" {
			in.City = "unknown"
		}
		return fmt.Sprintf(`{"city":"%s","weather":"sunny","temperature_c":26}`, in.City), nil
	case "fake_stock":
		var in struct {
			Symbol string `json:"symbol"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &in); err != nil {
			return "", fmt.Errorf("decode fake_stock args failed: %w", err)
		}
		if in.Symbol == "" {
			in.Symbol = "UNKNOWN"
		}
		return fmt.Sprintf(`{"symbol":"%s","price_usd":321.45}`, in.Symbol), nil
	default:
		return "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
	}
}

func TestModel(t *testing.T) {
	modelWithTools := mustNewTestModelWithTools(t)
	messages := buildTestMessages("")

	stream, err := modelWithTools.Stream(t.Context(), messages)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	result := HandleStream(t.Context(), stream)

outer:
	for {
		select {
		case <-result.Closed:
			if result.Err != nil {
				t.Fatalf("handle stream failed: %v", result.Err)
			} else {
				jb, _ := json.MarshalIndent(result.FinalResult, " ", " ")
				fmt.Println(string(jb))
			}
			break outer
		case content := <-result.Contents:
			t.Logf("content=%s, reasoning=%s", content.Content, content.ReasoningContent)
		}
	}
}

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
	HandleStreamWithCallback(context.Background(), stream, &Callbacks{
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
			HandleStreamWithCallback(context.Background(), stream, &Callbacks{
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
	HandleStreamWithCallback(context.Background(), stream, &Callbacks{
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
	messages := buildTestMessages("当你你在输出思考内容的时候，使用一句话来描述你正在思考什么。如果你输出工具调用，就不要输出任何文本回答内容，只需要进行工具调用")

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
		HandleStreamWithCallback(t.Context(), callbackStream, &Callbacks{
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
