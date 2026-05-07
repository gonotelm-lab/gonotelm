package chat

import (
	"encoding/json"
	"fmt"
	"os"
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

	model, err := New(t.Context(), &Config{
		Type: Openai,
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

func TestModelWithCallback(t *testing.T) {
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
		HandleStreamWithCallback(t.Context(), callbackStream, &Callbacks{
			OnReasoning: func(msg *schema.Message) {
				t.Logf("[callback] reasoning=%s", msg.ReasoningContent)
			},
			OnReasoningEnd: func(msg *schema.Message) {
				t.Logf("[callback] reasoning end, content=%s", msg.ReasoningContent)
			},
			OnContent: func(msg *schema.Message) {
				t.Logf("[callback] content=%s", msg.Content)
			},
			OnTooling: func(msg *schema.Message) {
				bb, _ := json.Marshal(msg.ToolCalls)
				t.Logf("[callback] tooling=%v", string(bb))
			},
			OnError: func(err error) {
				t.Errorf("callback stream failed: %v", err)
			},
			OnEnd: func(msg *schema.Message) {
				if msg == nil {
					t.Error("callback stream final result is nil")
					return
				}
				endMsg = msg
				t.Logf("[callback] end tooling total=%d", len(msg.ToolCalls))
				t.Logf("[callback] end content=%s", msg.Content)
				t.Logf("[callback] end reasoning=%s", msg.ReasoningContent)
				jb, _ := json.MarshalIndent(msg, " ", " ")
				fmt.Println(string(jb))
			},
		})
		callbackStream.Close()

		if endMsg == nil {
			t.Fatal("callback stream end message is nil")
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
