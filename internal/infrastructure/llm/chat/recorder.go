package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	Temperature float32  `json:"temperature,omitempty"`
	TopP        float32  `json:"top_p,omitempty"`
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
	InputParts       []*RecordInputPart       `json:"input_parts,omitempty"`
}

type RecordInputPart struct {
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
}

func toRecordInputPart(part schema.MessageInputPart) *RecordInputPart {
	r := &RecordInputPart{}
	switch part.Type {
	case schema.ChatMessagePartTypeText:
		r.Text = part.Text
	case schema.ChatMessagePartTypeImageURL:
		if part.Image != nil {
			if part.Image.URL != nil {
				if strings.HasPrefix(*part.Image.URL, "https") || strings.HasPrefix(*part.Image.URL, "https") {
					r.Image = "[Image URL]"
				} else if strings.HasPrefix(*part.Image.URL, "data") {
					r.Image = fmt.Sprintf("[Image Base64 Of Length %d]", len(*part.Image.URL))
				} else {
					r.Image = "[Unknown Image URL]"
				}
			} else if part.Image.Base64Data != nil {
				r.Image = fmt.Sprintf("[Image Base64 Of Length %d]", len(*part.Image.Base64Data))
			}
		}
	}

	return r
}

func toRecordInputParts(parts []schema.MessageInputPart) []*RecordInputPart {
	if len(parts) == 0 {
		return nil
	}

	ips := make([]*RecordInputPart, 0, len(parts))
	for _, part := range parts {
		ips = append(ips, toRecordInputPart(part))
	}
	return ips
}

func toRecordInputMessages(msgs []*schema.Message) []*RecordInputMessage {
	inputs := make([]*RecordInputMessage, 0, len(msgs))
	for _, msg := range msgs {
		im := &RecordInputMessage{
			Role:         string(msg.Role),
			Content:      msg.Content,
			ToolCallId:   msg.ToolCallID,
			ToolCallName: msg.ToolName,
			ToolCalls:    toToolCalls(msg.ToolCalls),
		}
		// truncate content if too long
		var truncated bool
		if im.Content != "" {
			im.Content, truncated = pkgstr.TruncateRuneV2(msg.Content, maxRecordToolResultRune)
			if truncated {
				im.Content = im.Content + " (...truncated)"
			}
		}

		im.InputParts = toRecordInputParts(msg.UserInputMultiContent)

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
