package pipeline

import (
	"context"
)

type Step interface {
	Name() string
	Execute(ctx context.Context, data *Data) error
}

type ConditionalStep interface {
	Step
	ShouldExecute(ctx context.Context, data *Data) bool
}

// stepFunc adapts a plain function into a Step.
type stepFunc struct {
	name string
	exec func(ctx context.Context, data *Data) error
}

func (s *stepFunc) Name() string { return s.name }

func (s *stepFunc) Execute(ctx context.Context, data *Data) error {
	return s.exec(ctx, data)
}

// NewStepFunc builds a Step from a name and an execute function.
func NewStepFunc(name string, exec func(ctx context.Context, data *Data) error) Step {
	return &stepFunc{name: name, exec: exec}
}

// conditionalStepFunc adapts a plain function plus a predicate into a
// ConditionalStep.
type conditionalStepFunc struct {
	stepFunc
	should func(ctx context.Context, data *Data) bool
}

func (s *conditionalStepFunc) ShouldExecute(ctx context.Context, data *Data) bool {
	if s.should == nil {
		return true
	}
	return s.should(ctx, data)
}

// NewConditionalStepFunc builds a Step that runs exec only when should reports
// true. A nil should means the step always executes.
func NewConditionalStepFunc(
	name string,
	should func(ctx context.Context, data *Data) bool,
	exec func(ctx context.Context, data *Data) error,
) Step {
	return &conditionalStepFunc{
		stepFunc: stepFunc{name: name, exec: exec},
		should:   should,
	}
}
