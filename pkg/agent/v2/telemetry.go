package agent

import "go.opentelemetry.io/otel/attribute"

const (
	spanAgentRun   = "agent.run"
	spanAgentRound = "agent.round"
	spanAgentTool  = "agent.tool"
)

const (
	attrAgentName             attribute.Key = "agent.name"
	attrAgentRound            attribute.Key = "agent.round"
	attrAgentMaxRound         attribute.Key = "agent.max_round"
	attrAgentToolName         attribute.Key = "agent.tool.name"
	attrAgentToolCallID       attribute.Key = "agent.tool.call_id"
	attrAgentToolDurationMs   attribute.Key = "agent.tool.duration_ms"
	attrAgentToolError        attribute.Key = "agent.tool.error"
	attrGenAIFinishReason     attribute.Key = "gen_ai.finish_reason"
	attrGenAIPromptTokens     attribute.Key = "gen_ai.usage.prompt_tokens"
	attrGenAICompletionTokens attribute.Key = "gen_ai.usage.completion_tokens"
)
