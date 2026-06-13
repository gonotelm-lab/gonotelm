package prompts

import "github.com/cloudwego/eino/schema"

const systemPrompt = "You are GoNoteLM, a powerful intelligent assistant. You will handle specific tasks based on the provided source content."

// 注入系统prompt
var gonotelmSystemPrompt = schema.SystemMessage(systemPrompt)

func prependSystemMessage(msgs []*schema.Message) []*schema.Message {
	return append([]*schema.Message{gonotelmSystemPrompt}, msgs...)
}
