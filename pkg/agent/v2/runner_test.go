package agent

import (
	"context"
	stderrors "errors"
	"testing"
)

func TestNewRunnerValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("nil model", func(t *testing.T) {
		if _, err := NewRunner[struct{}](ctx, RunnerConfig{}); err == nil {
			t.Fatal("expected error for nil model")
		}
	})

	t.Run("duplicate tool names", func(t *testing.T) {
		_, err := NewRunner[struct{}](ctx, RunnerConfig{
			Model: newScriptedModel(),
			Tools: []Tool{
				&fakeTool{name: "a"},
				&fakeTool{name: "a"},
			},
		})
		if err == nil {
			t.Fatal("expected duplicate tool error")
		}
	})

	t.Run("tool info error", func(t *testing.T) {
		_, err := NewRunner[struct{}](ctx, RunnerConfig{
			Model: newScriptedModel(),
			Tools: []Tool{&fakeTool{name: "a", infoErr: stderrors.New("boom")}},
		})
		if err == nil {
			t.Fatal("expected tool info error")
		}
	})

	t.Run("nil tool", func(t *testing.T) {
		_, err := NewRunner[struct{}](ctx, RunnerConfig{
			Model: newScriptedModel(),
			Tools: []Tool{nil},
		})
		if err == nil {
			t.Fatal("expected nil tool error")
		}
	})
}

func TestRunnerSortsAndBindsTools(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})

	runner, err := NewRunner[struct{}](ctx, RunnerConfig{
		Model: model,
		Tools: []Tool{
			&fakeTool{name: "b"},
			&fakeTool{name: "a"},
		},
	})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{}); err != nil {
		t.Fatalf("run: %v", err)
	}

	calls := model.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("model calls: got %d, want 1", len(calls))
	}
	want := []string{"a", "b"}
	if len(calls[0].tools) != len(want) {
		t.Fatalf("tools: got %v, want %v", calls[0].tools, want)
	}
	for i := range want {
		if calls[0].tools[i] != want[i] {
			t.Fatalf("tools: got %v, want %v", calls[0].tools, want)
		}
	}
}

func TestNewRunnerEmptyToolName(t *testing.T) {
	_, err := NewRunner[struct{}](context.Background(), RunnerConfig{
		Model: newScriptedModel(),
		Tools: []Tool{&fakeTool{name: ""}},
	})
	if err == nil {
		t.Fatal("expected empty tool name error")
	}
}

func TestRunnerBaseModel(t *testing.T) {
	model := newScriptedModel()
	runner := mustRunner(t, RunnerConfig{Model: model})
	if runner.BaseModel() != model {
		t.Fatal("BaseModel should return the configured model")
	}
}

func TestNewRunnerNilToolInfo(t *testing.T) {
	_, err := NewRunner[struct{}](context.Background(), RunnerConfig{
		Model: newScriptedModel(),
		Tools: []Tool{&fakeTool{name: "nil", nilInfo: true}},
	})
	if err == nil {
		t.Fatal("expected nil tool info error")
	}
}

func TestRunWithToolsError(t *testing.T) {
	model := &withToolsErrorModel{scriptedModel: newScriptedModel(
		scriptedResponse{msg: assistantMsg("ok", nil)},
	)}
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "echo"}},
	})

	if _, err := runner.Run(context.Background(), NewSession(struct{}{}), RunConfig[struct{}]{}); err == nil {
		t.Fatal("expected WithTools error")
	}
}

func TestNewStatelessRunner(t *testing.T) {
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner, err := NewStatelessRunner(context.Background(), RunnerConfig{Model: model})
	if err != nil {
		t.Fatalf("new stateless runner: %v", err)
	}

	session := NewStatelessSession()
	if session == nil || session.State != (NoState{}) {
		t.Fatalf("stateless session: %+v", session)
	}
	res, err := runner.Run(context.Background(), session, StatelessRunConfig{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Final.Content != "ok" {
		t.Fatalf("final: %+v", res.Final)
	}
}
