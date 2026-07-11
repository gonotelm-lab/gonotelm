package stream

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/llm"
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
		{name: "stop", reason: llm.FinishReasonStop, wantError: false, wantOnDone: true},
		{name: "length", reason: llm.FinishReasonLength, wantError: false, wantOnDone: true},
		{name: "tool_calls", reason: llm.FinishReasonToolCalls, wantError: true, wantOnDone: false, wantErrReason: StreamErrorReasonModelFinishReason},
		{name: "content_filter", reason: llm.FinishReasonContentFilter, wantError: true, wantOnDone: false, wantErrReason: StreamErrorReasonModelFinishReason},
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

func TestHandleStream(t *testing.T) {
	s := schema.StreamReaderFromArray([]*schema.Message{
		{
			Role:    schema.Assistant,
			Content: "hello",
		},
		{
			Role:    schema.Assistant,
			Content: " world",
		},
	})
	defer s.Close()

	result := HandleStream(context.Background(), s)

	var packedContents []*PackedContent
outer:
	for {
		select {
		case <-result.Closed:
			if result.Err != nil {
				t.Fatalf("handle stream failed: %v", result.Err)
			}
			if result.FinalResult == nil {
				t.Fatal("final result is nil")
			}
			if result.FinalResult.Content != "hello world" {
				t.Fatalf("unexpected content: got=%q want=%q", result.FinalResult.Content, "hello world")
			}
			break outer
		case content := <-result.Contents:
			packedContents = append(packedContents, content)
		}
	}

	if len(packedContents) != 2 {
		t.Fatalf("expected 2 packed contents, got=%d", len(packedContents))
	}
	if packedContents[0].Content != "hello" {
		t.Fatalf("unexpected first delta: got=%q want=%q", packedContents[0].Content, "hello")
	}
	if packedContents[1].Content != " world" {
		t.Fatalf("unexpected second delta: got=%q want=%q", packedContents[1].Content, " world")
	}

	fmt.Println(packedContents)
}
