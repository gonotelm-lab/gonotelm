package chat

import (
	"context"
	"encoding/json"
	"time"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgstr "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const (
	RecordMetaStreaming  = "streaming"
	RecordMetaThinking   = "thinking"
	RecordMetaJSONObject = "json_object"
)

const (
	maxRecordToolResultRune = 300
)

type RecordTokenUsage struct {
	PromptTokens       int // Total prompt input tokens, including cache hit and cache miss
	PromptCachedTokens int // cached input prompt tokens
	CompletionTokens   int // output tokens
	TotalTokens        int // PromptTokens + CompletionTokens
}

func toRecordTokenUsage(u *model.TokenUsage) *RecordTokenUsage {
	if u == nil {
		return nil
	}
	return &RecordTokenUsage{
		PromptTokens:       u.PromptTokens,
		PromptCachedTokens: u.PromptTokenDetails.CachedTokens,
		CompletionTokens:   u.CompletionTokens,
		TotalTokens:        u.TotalTokens,
	}
}

type ModelParameters struct {
	Model       string   `json:"model"`
	Temperature float32  `json:"temperature"`
	TopP        float32  `json:"top_p"`
	Stop        []string `json:"stop,omitempty"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
}

func toModelParameters(c *model.Config) *ModelParameters {
	if c == nil {
		return nil
	}

	return &ModelParameters{
		Model:       c.Model,
		Temperature: c.Temperature,
		TopP:        c.TopP,
		Stop:        c.Stop,
		MaxTokens:   c.MaxTokens,
	}
}

type Record struct {
	Provider   Provider             // model provider
	Scene      pkgcontext.SceneType // usage scene
	Input      []*RecordInputMessage
	InputTools []*RecordInputTool // tool definitions
	Output     *RecordOutputMessage
	Parameters *ModelParameters
	Usage      *RecordTokenUsage

	StartTime time.Time // the start time of the llm call
	EndTime   time.Time // the finish time of the llm call

	Metadatas map[string]any
	Error     error
}

type RecordInputMessage struct {
	Role             string                   `json:"role"`
	Content          string                   `json:"content,omitempty"`
	ReasoningContent string                   `json:"reasoning_content,omitempty"`
	ToolCalls        []*RecordMessageToolCall `json:"tool_calls,omitempty"`
	ToolCallId       string                   `json:"tool_call_id,omitempty"`
	ToolCallName     string                   `json:"tool_call_name,omitempty"`
}

func toRecordInputMessages(msgs []*schema.Message) []*RecordInputMessage {
	inputs := make([]*RecordInputMessage, 0, len(msgs))
	for _, m := range msgs {
		im := &RecordInputMessage{
			Role:         string(m.Role),
			Content:      m.Content,
			ToolCallId:   m.ToolCallID,
			ToolCallName: m.ToolName,
			ToolCalls:    toToolCalls(m.ToolCalls),
		}
		// truncate tool call results
		if m.Role == schema.Tool {
			// truncate tool call result content
			var truncated bool
			im.Content, truncated = pkgstr.TruncateRuneV2(m.Content, maxRecordToolResultRune)
			if truncated {
				im.Content = im.Content + " (...truncated)"
			}
		}

		inputs = append(inputs, im)
	}
	return inputs
}

type RecordInputTool struct {
	Name         string          `json:"name"`
	Desc         string          `json:"desc"`
	ParamsSchema json.RawMessage `json:"params_schema"`
}

func toRecordInputTool(tools []*schema.ToolInfo) []*RecordInputTool {
	ts := make([]*RecordInputTool, 0, len(tools))
	for _, t := range tools {
		tc := &RecordInputTool{
			Name: t.Name,
			Desc: t.Desc,
		}
		if t.ParamsOneOf != nil {
			if jschema, _ := t.ToJSONSchema(); jschema != nil {
				paramSchema, err := json.Marshal(jschema)
				if err == nil {
					tc.ParamsSchema = paramSchema
				}
			}
		}

		ts = append(ts, tc)
	}

	return ts
}

type RecordMessageToolCall struct {
	Id       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

func toToolCalls(fcs []schema.ToolCall) []*RecordMessageToolCall {
	rfcs := make([]*RecordMessageToolCall, 0, len(fcs))
	for _, toolCall := range fcs {
		rfc := &RecordMessageToolCall{
			Id:   toolCall.ID,
			Type: toolCall.Type,
			Function: FunctionCall{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		}

		rfcs = append(rfcs, rfc)
	}

	return rfcs
}

type RecordOutputMessage struct {
	Role             string                   `json:"role"`
	Content          string                   `json:"content,omitempty"`
	ReasoningContent string                   `json:"reasoning_content,omitempty"`
	ToolCalls        []*RecordMessageToolCall `json:"tool_calls,omitempty"`
	FinishReason     string                   `json:"finish_reason,omitempty"`
}

func toRecordOutputMessage(msg *schema.Message) *RecordOutputMessage {
	m := &RecordOutputMessage{
		Role:             string(msg.Role),
		ReasoningContent: msg.ReasoningContent,
		Content:          msg.Content,
		ToolCalls:        toToolCalls(msg.ToolCalls),
	}
	if msg.ResponseMeta != nil {
		m.FinishReason = msg.ResponseMeta.FinishReason
	}

	return m
}

// Recorder records the calling of LLM, such as token usage, billing stats
type Recorder interface {
	Record(ctx context.Context, rec *Record) error
}

func buildRecord(ctx context.Context, endTime time.Time) *Record {
	metadatas := make(map[string]any)
	metadatas[RecordMetaStreaming] = getStreaming(ctx)
	if thinking, ok := getThinking(ctx); ok {
		metadatas[RecordMetaThinking] = thinking
	}
	metadatas[RecordMetaJSONObject] = getJSONObject(ctx)

	return &Record{
		Provider:  getProvider(ctx),
		Scene:     pkgcontext.GetSceneType(ctx),
		StartTime: getStartTime(ctx),
		EndTime:   endTime,
		Metadatas: metadatas,
	}
}

func buildEndRecord(
	ctx context.Context,
	input *model.CallbackInput,
	output *model.CallbackOutput,
	endTime time.Time,
) *Record {
	r := buildRecord(ctx, endTime)
	r.Input = toRecordInputMessages(input.Messages)
	r.InputTools = toRecordInputTool(input.Tools)
	r.Parameters = toModelParameters(input.Config) // same as output.Config
	r.Output = toRecordOutputMessage(output.Message)
	r.Usage = toRecordTokenUsage(output.TokenUsage)

	return r
}

func buildErrorRecord(ctx context.Context, err error, endTime time.Time) *Record {
	r := buildRecord(ctx, endTime)
	r.Error = err

	return r
}
