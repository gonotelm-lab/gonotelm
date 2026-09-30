package pipeline

import (
	"context"
	"fmt"
	"log/slog"
)

type Pipeline struct {
	steps []Step
	name  string
}

func New(name string) *Pipeline {
	return &Pipeline{
		steps: make([]Step, 0),
		name:  name,
	}
}

func (p *Pipeline) AddStep(step Step) {
	p.steps = append(p.steps, step)
}

func (p *Pipeline) AddSteps(steps ...Step) {
	p.steps = append(p.steps, steps...)
}

func (p *Pipeline) Name() string {
	return p.name
}

func (p *Pipeline) Execute(ctx context.Context, data *Data) error {
	slog.DebugContext(ctx, fmt.Sprintf("pipeline %s executing with %d steps", p.name, len(p.steps)))

	for idx, step := range p.steps {
		// check if step should be executed
		if conditionalStep, ok := step.(ConditionalStep); ok {
			if !conditionalStep.ShouldExecute(ctx, data) {
				slog.DebugContext(ctx, fmt.Sprintf("pipeline %s step %d (%s) skipped", p.name, idx, step.Name()))
				continue
			}
		}

		slog.DebugContext(ctx, fmt.Sprintf("pipeline %s step %d (%s) is running", p.name, idx, step.Name()))
		if err := step.Execute(ctx, data); err != nil {
			return err
		}

		slog.DebugContext(ctx, fmt.Sprintf("pipeline %s step %d (%s) completed", p.name, idx, step.Name()))
	}

	slog.DebugContext(ctx, fmt.Sprintf("pipeline %s completed", p.name))

	return nil
}
