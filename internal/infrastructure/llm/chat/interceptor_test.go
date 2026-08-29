package chat

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const citeArgsJSON = `{"source_doc_ids":["doc-1"]}`

func newToolCallChunk(id, args string) *schema.Message {
	idx := 0 // ConcatMessages merges tool-call arg deltas by Index
	return &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			Index: &idx,
			ID:    id,
			Type:  "function",
			Function: schema.FunctionCall{
				Name:      "CiteSourceDoc",
				Arguments: args,
			},
		}},
	}
}

// TestSharedCallbackOutputWritebackDoublesToolArgs documents the anti-pattern:
// writing finalMsg back onto a shared lastChunk.Message makes the agent's second
// ConcatMessages produce exactly doubled arguments (pre-fix root cause).
func TestSharedCallbackOutputWritebackDoublesToolArgs(t *testing.T) {
	m1 := newToolCallChunk("call_1", citeArgsJSON)
	m2 := newToolCallChunk("call_1", "") // finish frame with no arg delta

	co1 := &model.CallbackOutput{Message: m1}
	co2 := &model.CallbackOutput{Message: m2}

	// Agent already received the first frame (Message pointer taken at convert time).
	agentMsgs := []*schema.Message{co1.Message}

	// Interceptor concatenates all frames and writes back onto the shared last CallbackOutput.
	interceptorMsgs := []*schema.Message{co1.Message, co2.Message}
	finalMsg, err := schema.ConcatMessages(interceptorMsgs)
	require.NoError(t, err)
	require.Len(t, finalMsg.ToolCalls, 1)
	assert.Equal(t, citeArgsJSON, finalMsg.ToolCalls[0].Function.Arguments,
		"interceptor Concat result should be a single clean JSON")

	lastChunk := co2
	lastChunk.Message = finalMsg // mirrors the old interceptor.go write-back

	// Agent Recvs the last frame after write-back; convert now sees finalMsg.
	agentMsgs = append(agentMsgs, co2.Message)
	agentFinal, err := schema.ConcatMessages(agentMsgs)
	require.NoError(t, err)
	require.Len(t, agentFinal.ToolCalls, 1)

	assert.Equal(t, citeArgsJSON+citeArgsJSON, agentFinal.ToolCalls[0].Function.Arguments,
		"agent's second Concat should produce exactly doubled arguments")
	assert.Equal(t, citeArgsJSON, finalMsg.ToolCalls[0].Function.Arguments,
		"interceptor record stays single-copy (matches clean llm_logs vs doubled Redis)")
}

type captureRecorder struct {
	mu   sync.Mutex
	done chan struct{}
	once sync.Once
	args string
}

func newCaptureRecorder() *captureRecorder {
	return &captureRecorder{done: make(chan struct{})}
}

func (r *captureRecorder) Record(_ context.Context, rec *Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec.Output != nil && len(rec.Output.ToolCalls) > 0 {
		r.args = rec.Output.ToolCalls[0].Function.Arguments
	}
	r.once.Do(func() { close(r.done) })
	return nil
}

func (r *captureRecorder) wait(t *testing.T) {
	t.Helper()
	select {
	case <-r.done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for interceptor record")
	}
}

func (r *captureRecorder) recordedArgs() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.args
}

// TestOnEndWithStreamOutputDoesNotMutateSharedCallbackOutput is a regression test:
// after Copy + OnEndWithStreamOutput records (before the agent drains the stream),
// shared frames must not be mutated.
func TestOnEndWithStreamOutputDoesNotMutateSharedCallbackOutput(t *testing.T) {
	m1 := newToolCallChunk("call_1", citeArgsJSON)
	m2 := newToolCallChunk("call_1", "")
	co1 := &model.CallbackOutput{Message: m1}
	co2 := &model.CallbackOutput{Message: m2}

	parent, writer := schema.Pipe[callbacks.CallbackOutput](2)
	copies := parent.Copy(2)
	interceptorSR, agentSR := copies[0], copies[1]

	rec := newCaptureRecorder()
	interceptor := newInterceptor(context.Background(), rec)

	runInfo := &callbacks.RunInfo{
		Name:      "test",
		Type:      "ChatModel",
		Component: "ChatModel",
	}
	// recordEnd → buildEndRecord needs CallbackInput from OnStart.
	ctx := interceptor.OnStart(context.Background(), runInfo, &model.CallbackInput{
		Messages: []*schema.Message{},
	})

	// Start interceptor (DetachGo consumes asynchronously).
	interceptor.OnEndWithStreamOutput(ctx, runInfo, interceptorSR)

	// Produce two frames then close (same as deepseek stream end). Send returns true if closed.
	require.False(t, writer.Send(co1, nil))
	require.False(t, writer.Send(co2, nil))
	writer.Close()

	// Wait until interceptor Concat + record finishes (pre-fix wrote back lastChunk.Message here).
	rec.wait(t)
	assert.Equal(t, citeArgsJSON, rec.recordedArgs(), "llm_logs side should stay a single clean copy")
	assert.Same(t, m2, co2.Message, "shared CallbackOutput.Message must not be replaced with finalMsg")

	// Agent path: mimic deepseek convert — each Recv reads current CallbackOutput.Message.
	var agentMsgs []*schema.Message
	for {
		out, err := agentSR.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		co := model.ConvCallbackOutput(out)
		require.NotNil(t, co)
		require.NotNil(t, co.Message)
		agentMsgs = append(agentMsgs, co.Message)
	}
	agentSR.Close()

	agentFinal, err := schema.ConcatMessages(agentMsgs)
	require.NoError(t, err)
	require.Len(t, agentFinal.ToolCalls, 1)

	assert.Equal(t, citeArgsJSON, agentFinal.ToolCalls[0].Function.Arguments,
		"after fix, agent Concat should keep a single copy of args")
	assert.Equal(t, citeArgsJSON, rec.recordedArgs())
}

// TestAgentConcatWithoutWritebackStaysClean is the control case: without write-back,
// agent Concat does not double arguments.
func TestAgentConcatWithoutWritebackStaysClean(t *testing.T) {
	m1 := newToolCallChunk("call_1", citeArgsJSON)
	m2 := newToolCallChunk("call_1", "")
	co1 := &model.CallbackOutput{Message: m1}
	co2 := &model.CallbackOutput{Message: m2}

	agentMsgs := []*schema.Message{co1.Message, co2.Message}
	finalMsg, err := schema.ConcatMessages([]*schema.Message{co1.Message, co2.Message})
	require.NoError(t, err)
	// Intentionally no write-back: record via local finalMsg only.
	_ = finalMsg

	agentFinal, err := schema.ConcatMessages(agentMsgs)
	require.NoError(t, err)
	require.Len(t, agentFinal.ToolCalls, 1)
	assert.Equal(t, citeArgsJSON, agentFinal.ToolCalls[0].Function.Arguments)
	assert.Equal(t, citeArgsJSON, finalMsg.ToolCalls[0].Function.Arguments)
}
